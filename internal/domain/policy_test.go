package domain

import (
	"testing"
)

func TestPolicy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		policy  Policy
		wantErr bool
	}{
		{
			name: "valid policy",
			policy: Policy{
				ID:       "pol-1",
				TenantID: "tenant-A",
				FilesystemRules: []string{
					"allow:/workspace",
				},
				NetworkRules: []string{
					"deny:0.0.0.0/0",
				},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			policy: Policy{
				TenantID: "tenant-A",
			},
			wantErr: true,
		},
		{
			name: "missing tenant ID",
			policy: Policy{
				ID: "pol-1",
			},
			wantErr: true,
		},
		{
			name: "empty filesystem rule",
			policy: Policy{
				ID:       "pol-1",
				TenantID: "tenant-A",
				FilesystemRules: []string{
					"allow:/workspace",
					"",
				},
			},
			wantErr: true,
		},
		{
			name: "empty network rule",
			policy: Policy{
				ID:       "pol-1",
				TenantID: "tenant-A",
				NetworkRules: []string{
					"",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Policy.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
