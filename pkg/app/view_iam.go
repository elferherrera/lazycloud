package app

import (
	"fmt"
	"strings"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func iamMemberType(member string) string {
	if i := strings.Index(member, ":"); i > 0 {
		return member[:i]
	}
	return member
}

func iamBindingRows(bindings []providers.IAMBinding) [][]string {
	rows := make([][]string, 0, len(bindings))
	for _, b := range bindings {
		rows = append(rows, []string{
			b.Member, iamMemberType(b.Member), b.Role, b.Condition, formatUpdatedAt(b.UpdatedAt),
		})
	}
	return rows
}

func iamBindingRowsAll(bindings []providers.IAMBinding) [][]string {
	rows := iamBindingRows(bindings)
	for i, b := range bindings {
		rows[i] = append(rows[i], b.ProjectID)
	}
	return rows
}

func iamBindingResource(a *app, projectID, member, role string) (any, error) {
	binding, err := a.cache.GetIAMBinding(projectID, member, role)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, fmt.Errorf("IAM binding not found in cache")
	}
	return binding, nil
}

func fetchProjectIAM(a *app, projectID string) (string, error) {
	bindings, err := providers.ListProjectIAMPolicy(projectID)
	if err != nil {
		return "", err
	}
	if err := a.cache.SaveIAMBindings(projectID, bindings); err != nil {
		return "", err
	}
	a.logger.Update("iam", projectID, len(bindings))
	return fmt.Sprintf("Fetched %d IAM bindings", len(bindings)), nil
}

var iamView = viewDef{
	title:   "IAM policy",
	headers: []string{"Member", "Type", "Role", "Condition", "Updated"},
	load: func(a *app) ([][]string, error) {
		bindings, err := a.cache.ListIAMBindings(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return iamBindingRows(bindings), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d IAM bindings for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewIAM, "IAM policy", fetchProjectIAM),
	fetch:   fetchProjectIAM,
	resource: func(a *app, row []string) (any, error) {
		return iamBindingResource(a, appState.ProjectID, row[0], row[2])
	},
	samples: []any{providers.IAMBinding{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPIAMURL(appState.ProjectID)
	},
}

var iamAllView = viewDef{
	title:   "IAM policy",
	headers: []string{"Member", "Type", "Role", "Condition", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		bindings, err := a.cache.ListAllIAMBindings()
		if err != nil {
			return nil, err
		}
		return iamBindingRowsAll(bindings), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d IAM bindings for all projects", count)
	},
	refresh: func(a *app) {
		a.statusInfo("Fetching IAM policy for all projects...")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("iam", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusInfo(fmt.Sprintf("Fetching IAM policy for %s...", p.ProjectID))
				})
				bindings, err := providers.ListProjectIAMPolicy(p.ProjectID)
				if err != nil {
					a.reportError("iam", p.ProjectID, "Failed to fetch IAM policy for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveIAMBindings(p.ProjectID, bindings); err != nil {
					a.reportError("iam", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("iam", p.ProjectID, len(bindings))
				total += len(bindings)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewIAMAll)
				status := fmt.Sprintf("[green]Fetched %d IAM bindings across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return iamBindingResource(a, row[5], row[0], row[2])
	},
	samples: []any{providers.IAMBinding{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPIAMURL(a.info.GetCell(row, 5).Text)
	},
}

func showIAM(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(iamViewKind())
}
