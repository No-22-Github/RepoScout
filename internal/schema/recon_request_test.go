package schema

import (
	"testing"
)

func TestParseReconRequest(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid minimal request",
			json: `{
				"task": "Add authentication",
				"repo_root": "/home/user/project"
			}`,
			wantErr: false,
		},
		{
			name: "valid full request",
			json: `{
				"task": "Add a new REST API endpoint for user authentication",
				"repo_root": "/path/to/repository",
				"profile": "feature-add",
				"seed_files": ["cmd/server/main.go", "internal/auth/handler.go"],
				"focus_symbols": ["LoginHandler", "AuthMiddleware"],
				"focus_checks": ["security", "error-handling"],
				"budget": {
					"max_files": 100,
					"max_tokens": 50000,
					"max_time_sec": 120
				}
			}`,
			wantErr: false,
		},
		{
			name:    "missing task",
			json:    `{"repo_root": "/home/user/project"}`,
			wantErr: true,
			errMsg:  "task is required",
		},
		{
			name:    "missing repo_root",
			json:    `{"task": "Add authentication"}`,
			wantErr: true,
			errMsg:  "repo_root is required",
		},
		{
			name:    "empty json",
			json:    `{}`,
			wantErr: true,
			errMsg:  "task is required",
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantErr: true,
			errMsg:  "failed to parse recon request",
		},
		{
			name: "negative budget max_files",
			json: `{
				"task": "Add authentication",
				"repo_root": "/home/user/project",
				"budget": {"max_files": -1}
			}`,
			wantErr: true,
			errMsg:  "budget.max_files cannot be negative",
		},
		{
			name: "negative budget max_tokens",
			json: `{
				"task": "Add authentication",
				"repo_root": "/home/user/project",
				"budget": {"max_tokens": -100}
			}`,
			wantErr: true,
			errMsg:  "budget.max_tokens cannot be negative",
		},
		{
			name: "negative budget max_time_sec",
			json: `{
				"task": "Add authentication",
				"repo_root": "/home/user/project",
				"budget": {"max_time_sec": -30}
			}`,
			wantErr: true,
			errMsg:  "budget.max_time_sec cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseReconRequest([]byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseReconRequest() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseReconRequest() error = %v, want containing %v", err, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseReconRequest() unexpected error: %v", err)
				return
			}
			if req == nil {
				t.Error("ParseReconRequest() returned nil request")
			}
		})
	}
}

func TestReconRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *ReconRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			req: &ReconRequest{
				Task:     "Add feature",
				RepoRoot: "/home/user/project",
			},
			wantErr: false,
		},
		{
			name: "empty task",
			req: &ReconRequest{
				RepoRoot: "/home/user/project",
			},
			wantErr: true,
			errMsg:  "task is required",
		},
		{
			name: "empty repo_root",
			req: &ReconRequest{
				Task: "Add feature",
			},
			wantErr: true,
			errMsg:  "repo_root is required",
		},
		{
			name: "valid budget",
			req: &ReconRequest{
				Task:     "Add feature",
				RepoRoot: "/home/user/project",
				Budget: &Budget{
					MaxFiles:   100,
					MaxTokens:  50000,
					MaxTimeSec: 120,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want containing %v", err, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestReconRequest_ToJSON(t *testing.T) {
	req := &ReconRequest{
		Task:      "Add authentication",
		RepoRoot:  "/home/user/project",
		Profile:   "feature-add",
		SeedFiles: []string{"main.go"},
		Budget:    &Budget{MaxFiles: 50},
	}

	data, err := req.ToJSON()
	if err != nil {
		t.Errorf("ToJSON() unexpected error: %v", err)
		return
	}

	// Verify we can parse it back
	parsed, err := ParseReconRequest(data)
	if err != nil {
		t.Errorf("Failed to parse ToJSON output: %v", err)
		return
	}

	if parsed.Task != req.Task {
		t.Errorf("Task mismatch: got %v, want %v", parsed.Task, req.Task)
	}
	if parsed.RepoRoot != req.RepoRoot {
		t.Errorf("RepoRoot mismatch: got %v, want %v", parsed.RepoRoot, req.RepoRoot)
	}
	if parsed.Profile != req.Profile {
		t.Errorf("Profile mismatch: got %v, want %v", parsed.Profile, req.Profile)
	}
	if len(parsed.SeedFiles) != len(req.SeedFiles) {
		t.Errorf("SeedFiles length mismatch: got %v, want %v", len(parsed.SeedFiles), len(req.SeedFiles))
	}
	if parsed.Budget == nil || parsed.Budget.MaxFiles != req.Budget.MaxFiles {
		t.Errorf("Budget.MaxFiles mismatch")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
