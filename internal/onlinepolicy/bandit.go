package onlinepolicy

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

type candidateArmEval struct {
	model            router.ModelDefinition
	expectedUtility  float64
	ucbScore         float64
	uncertainty      float64
	confidence       float64
	predictedQuality float64
	predictedLatency float64
	predictedCost    float64
	predictedSuccess float64
	scoreBreakdown   router.ScoreBreakdown
}

// ExplorationTracker enforces rolling window global and per-model exploration budgets.
type ExplorationTracker struct {
	mu           sync.RWMutex
	windowSize   int
	globalWindow []bool
	modelWindows map[string][]bool
}

// NewExplorationTracker initializes an ExplorationTracker with a given window size.
func NewExplorationTracker(windowSize int) *ExplorationTracker {
	if windowSize <= 0 {
		windowSize = 200
	}
	return &ExplorationTracker{
		windowSize:   windowSize,
		globalWindow: make([]bool, 0, windowSize),
		modelWindows: make(map[string][]bool),
	}
}

// CanExplore checks if global and per-model budgets permit an exploration decision.
func (t *ExplorationTracker) CanExplore(modelID string, globalLimit, perModelLimit int) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 1. Check Global Window
	globalExplores := 0
	for _, isExp := range t.globalWindow {
		if isExp {
			globalExplores++
		}
	}
	if globalExplores >= globalLimit {
		return false
	}

	// 2. Check Per-Model Window
	mWindow, ok := t.modelWindows[modelID]
	if ok {
		mExplores := 0
		for _, isExp := range mWindow {
			if isExp {
				mExplores++
			}
		}
		if mExplores >= perModelLimit {
			return false
		}
	}

	return true
}

// RecordDecision appends a decision to the rolling windows.
func (t *ExplorationTracker) RecordDecision(modelID string, wasExploration bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.globalWindow = append(t.globalWindow, wasExploration)
	if len(t.globalWindow) > t.windowSize {
		t.globalWindow = t.globalWindow[1:]
	}

	mWindow, ok := t.modelWindows[modelID]
	if !ok {
		mWindow = make([]bool, 0, t.windowSize)
	}
	mWindow = append(mWindow, wasExploration)
	if len(mWindow) > t.windowSize {
		mWindow = mWindow[1:]
	}
	t.modelWindows[modelID] = mWindow
}

// CurrentExplorationRate returns the active rolling exploration fraction.
func (t *ExplorationTracker) CurrentExplorationRate() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.globalWindow) == 0 {
		return 0.0
	}
	explores := 0
	for _, isExp := range t.globalWindow {
		if isExp {
			explores++
		}
	}
	return float64(explores) / float64(len(t.globalWindow))
}

// SelectArm evaluates the contextual Upper Confidence Bound (UCB) and executes safe Explore/Exploit.
func SelectArm(
	req router.RoutingRequest,
	feasibleModels []router.ModelDefinition,
	rejections []router.ModelRejection,
	store adaptive.LearningStore,
	prior adaptive.BetaPrior,
	state *PolicyState,
	tracker *ExplorationTracker,
	rng *rand.Rand,
) (PolicyDecision, error) {
	if len(feasibleModels) == 0 {
		return PolicyDecision{
			TaskID:             req.TaskID,
			RunID:              req.RunID,
			PolicyVersion:      state.Version,
			RewardVersion:      RewardVersion,
			ExplorationVersion: ExplorationVersion,
			Rejections:         rejections,
			DecidedAt:          time.Now().UTC(),
		}, fmt.Errorf("no feasible models available for policy selection")
	}

	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	costBudget := req.CostBudgetUSD
	if costBudget <= 0 {
		costBudget = 0.02
	}
	slaMs := req.LatencySLAMs
	if slaMs <= 0 {
		slaMs = 1000.0
	}

	candidates := make([]candidateArmEval, len(feasibleModels))

	// 1. Evaluate Stage 18 Empirical Predictions & Uncertainty for each feasible arm
	for i, m := range feasibleModels {
		key := adaptive.ComputeFeatureKey(req, m.ID)
		var profile adaptive.PerformanceProfile
		if store != nil {
			profile, _ = store.GetHierarchicalProfile(key)
		} else {
			profile = adaptive.PerformanceProfile{Key: key}
		}

		nominalQ := router.GetTaskQuality(m, req.TaskComplexity)
		pSuccess, _ := adaptive.PredictSuccess(profile, prior)
		qEst := adaptive.PredictQuality(profile, nominalQ)
		lEst := adaptive.PredictLatency(m, profile, req.EstimatedInputTokens, req.EstimatedOutputTokens)
		cEst := adaptive.PredictCost(m, req.EstimatedInputTokens, req.EstimatedOutputTokens, profile)

		n := float64(profile.TotalAttempts)
		confidence := n / (n + 10.0)

		// Dimensionless Normalized Utility
		cNorm := math.Min(1.0, cEst.PredictedUSD/costBudget)
		lNorm := math.Min(1.0, lEst.PredictedMs/slaMs)

		uExpected := state.RewardWeights.WeightQuality*qEst.Mean +
			state.RewardWeights.WeightSuccess*pSuccess -
			state.RewardWeights.WeightCost*cNorm -
			state.RewardWeights.WeightLatency*lNorm

		// Total Dimensionless Uncertainty
		tailRisk := 0.0
		if lEst.ObservedP50Ms > 0 {
			tailRisk = (lEst.ObservedP95Ms - lEst.ObservedP50Ms) / lEst.ObservedP50Ms
		}
		sigma := math.Sqrt(qEst.Variance + (1.0-pSuccess)*0.10 + math.Min(1.0, tailRisk*0.10))

		// UCB Score: Expected Utility + Exploration Bonus
		ucb := uExpected + state.ExplorationLambda*sigma

		candidates[i] = candidateArmEval{
			model:            m,
			expectedUtility:  uExpected,
			ucbScore:         ucb,
			uncertainty:      sigma,
			confidence:       confidence,
			predictedQuality: qEst.Mean,
			predictedLatency: lEst.PredictedMs,
			predictedCost:    cEst.PredictedUSD,
			predictedSuccess: pSuccess,
			scoreBreakdown: router.ScoreBreakdown{
				Quality:     qEst.Mean,
				Cost:        1.0 - cNorm,
				Latency:     1.0 - lNorm,
				Reliability: pSuccess,
			},
		}
	}

	// 2. Determine Best Exploit and Best Explore Candidates
	var bestExploit candidateArmEval
	var bestExplore candidateArmEval
	maxUtil := -math.MaxFloat64
	maxUCB := -math.MaxFloat64

	for _, c := range candidates {
		if c.expectedUtility > maxUtil {
			maxUtil = c.expectedUtility
			bestExploit = c
		}
		if c.ucbScore > maxUCB {
			maxUCB = c.ucbScore
			bestExplore = c
		}
	}

	// 3. Decide Mode: Exploit vs Explore
	decisionMode := DecisionExploit
	selected := bestExploit

	if bestExplore.model.ID != bestExploit.model.ID && bestExplore.confidence < 0.80 {
		if tracker != nil && tracker.CanExplore(bestExplore.model.ID, state.GlobalExplorationLimit, state.PerModelExplorationLimit) {
			if rng.Float64() < state.ExplorationBudget {
				decisionMode = DecisionExplore
				selected = bestExplore
			}
		}
	}

	if tracker != nil {
		tracker.RecordDecision(selected.model.ID, decisionMode == DecisionExplore)
	}

	// 4. Sort Fallback Candidates Deterministically
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].model.ID == selected.model.ID {
			return true
		}
		if candidates[j].model.ID == selected.model.ID {
			return false
		}
		return candidates[i].expectedUtility > candidates[j].expectedUtility
	})

	fallbacks := make([]string, 0, len(candidates)-1)
	for _, c := range candidates {
		if c.model.ID != selected.model.ID {
			fallbacks = append(fallbacks, c.model.ID)
		}
	}

	expRate := 0.0
	if tracker != nil {
		expRate = tracker.CurrentExplorationRate()
	}

	return PolicyDecision{
		TaskID:             req.TaskID,
		RunID:              req.RunID,
		SelectedModelID:    selected.model.ID,
		SelectedTier:       selected.model.Tier,
		DecisionMode:       decisionMode,
		PolicyVersion:      state.Version,
		RewardVersion:      RewardVersion,
		ExplorationVersion: ExplorationVersion,
		ExpectedUtility:    selected.expectedUtility,
		UCBScore:           selected.ucbScore,
		Uncertainty:        selected.uncertainty,
		ExplorationRate:    expRate,
		FallbackCandidates: fallbacks,
		Rejections:         rejections,
		ScoreBreakdown:     selected.scoreBreakdown,
		DecidedAt:          time.Now().UTC(),
	}, nil
}
