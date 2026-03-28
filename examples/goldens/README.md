# Goldens Dataset

This directory contains golden test samples for evaluating RepoScout's recall performance.

## Directory Structure

```
goldens/
├── README.md                    # This file
├── 001-browser-settings-toggle/ # Sample 1: Browser settings toggle task
│   ├── meta.json               # Task description and metadata
│   ├── recon_request.json      # The ReconRequest input
│   └── expected_files.json     # Expected relevant files (ground truth)
├── 002-auth-endpoint/          # Sample 2: Authentication endpoint task
│   ├── meta.json
│   ├── recon_request.json
│   └── expected_files.json
└── ...                         # More samples
```

## Sample Naming Convention

- Use numeric prefixes with 3 digits: `001-`, `002-`, `003-`, etc.
- Follow with a short kebab-case description of the task
- Example: `001-browser-settings-toggle`, `015-user-authentication`

## File Format

### meta.json

Contains task metadata and description.

```json
{
  "id": "001",
  "name": "browser-settings-toggle",
  "description": "Add a toggle switch to browser settings page",
  "repo_family": "browser-settings",
  "profile": "feature-add",
  "created_at": "2026-03-29",
  "difficulty": "medium",
  "notes": "Optional notes about this sample"
}
```

Required fields:
- `id`: Unique sample identifier
- `name`: Short name matching directory name
- `description`: Human-readable task description
- `repo_family`: The technology/framework family (for categorization)
- `profile`: The analysis profile to use

Optional fields:
- `created_at`: Creation date
- `difficulty`: Difficulty level (easy/medium/hard)
- `notes`: Additional context or notes

### recon_request.json

The standard ReconRequest input for RepoScout.

```json
{
  "task": "Add a toggle switch for dark mode in browser settings",
  "repo_root": "/path/to/repository",
  "profile": "feature-add",
  "seed_files": [
    "chrome/browser/ui/settings/settings_page.cc"
  ],
  "focus_symbols": ["SettingsPage", "TogglePreference"],
  "focus_checks": ["ui", "preferences"],
  "budget": {
    "max_files": 50,
    "max_time_sec": 60
  }
}
```

Required fields:
- `task`: The task description
- `repo_root`: Repository root path (can be placeholder for evaluation)

### expected_files.json

Ground truth of files that should be found relevant for the task.

```json
{
  "main_chain": [
    "chrome/browser/ui/settings/settings_page.cc",
    "chrome/browser/ui/settings/settings_page.h",
    "chrome/browser/preferences/pref_service.cc"
  ],
  "companion_files": [
    "chrome/browser/ui/settings/settings_toggle.cc",
    "chrome/browser/preferences/scoped_pref_map.cc"
  ],
  "optional_files": [
    "chrome/browser/ui/settings/settings_test.cc"
  ],
  "excluded_files": [
    "components/crash/core/common/crash_key.cc"
  ],
  "notes": "main_chain files are must-have, companion_files are nice-to-have"
}
```

Required fields:
- `main_chain`: Files that MUST be included in results (primary relevance)

Optional fields:
- `companion_files`: Files that SHOULD be included if possible (secondary relevance)
- `optional_files`: Files that MAY be included (tertiary relevance)
- `excluded_files`: Files that should NOT be included (anti-patterns)
- `notes`: Additional context

## How to Add a New Sample

1. **Create a new directory** with a numbered prefix:
   ```
   mkdir examples/goldens/003-your-task-name
   ```

2. **Create meta.json**:
   ```json
   {
     "id": "003",
     "name": "your-task-name",
     "description": "Description of the task",
     "repo_family": "relevant-tech-stack",
     "profile": "feature-add|bug-fix|refactor|understand",
     "created_at": "2026-03-29",
     "difficulty": "easy|medium|hard"
   }
   ```

3. **Create recon_request.json**:
   - Define the task, repo_root, seed_files, and other parameters
   - Use realistic values based on actual codebase patterns

4. **Create expected_files.json**:
   - List files that should be found relevant
   - Focus on `main_chain` (must-have files)
   - Add `companion_files` for secondary relevance
   - Optionally add `optional_files` and `excluded_files`

5. **Validate**:
   - Ensure all JSON files are valid
   - Ensure the sample follows the naming convention
   - Ensure the description is clear and actionable

## Evaluation Metrics

When evaluating RepoScout against these goldens:

- **Recall@N**: Percentage of expected files found in top N results
- **Main Chain Recall**: Percentage of `main_chain` files found
- **Companion Recall**: Percentage of `companion_files` files found
- **Precision@N**: Percentage of top N results that are relevant

## Notes

- The `repo_root` in recon_request.json may be a placeholder path
- The actual evaluation requires the corresponding repository to exist
- For CI testing, consider creating minimal mock repositories
