package app

import (
	"fmt"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func serviceAccountRows(accounts []providers.ServiceAccount) [][]string {
	rows := make([][]string, 0, len(accounts))
	for _, sa := range accounts {
		rows = append(rows, []string{
			sa.Email, sa.DisplayName, sa.UniqueID, yesNo(sa.Disabled), formatUpdatedAt(sa.UpdatedAt),
		})
	}
	return rows
}

func serviceAccountRowsAll(accounts []providers.ServiceAccount) [][]string {
	rows := make([][]string, 0, len(accounts))
	for _, sa := range accounts {
		rows = append(rows, []string{
			sa.Email, sa.DisplayName, sa.UniqueID, yesNo(sa.Disabled), formatUpdatedAt(sa.UpdatedAt), sa.ProjectID,
		})
	}
	return rows
}

func serviceAccountResource(a *app, projectID, email string) (any, error) {
	sa, err := a.cache.GetServiceAccount(projectID, email)
	if err != nil {
		return nil, err
	}
	if sa == nil {
		return nil, fmt.Errorf("service account not found in cache")
	}
	return sa, nil
}

var accountsView = viewDef{
	title:   "Service Accounts",
	headers: []string{"Email", "Display Name", "Unique ID", "Disabled", "Updated"},
	load: func(a *app) ([][]string, error) {
		accounts, err := a.cache.ListServiceAccounts(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return serviceAccountRows(accounts), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d service accounts for %s[-]", count, appState.ProjectID)
	},
	refresh: func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusText("[yellow]Fetching service accounts...[-]")
		go func() {
			projectID := appState.ProjectID
			accounts, err := providers.ListProjectServiceAccounts(projectID)
			if err != nil {
				a.reportError("accounts", projectID, "Failed to fetch service accounts", err)
				return
			}
			if err := a.cache.SaveServiceAccounts(projectID, accounts); err != nil {
				a.reportError("accounts", projectID, "Failed to update cache", err)
				return
			}
			a.logger.Update("accounts", projectID, len(accounts))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewAccounts)
				a.statusText(fmt.Sprintf("[green]Fetched %d service accounts[-]", len(accounts)))
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return serviceAccountResource(a, appState.ProjectID, row[0])
	},
	samples: []any{providers.ServiceAccount{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPServiceAccountURL(appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
}

var accountsAllView = viewDef{
	title:   "Service Accounts",
	headers: []string{"Email", "Display Name", "Unique ID", "Disabled", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		accounts, err := a.cache.ListAllServiceAccounts()
		if err != nil {
			return nil, err
		}
		return serviceAccountRowsAll(accounts), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("[yellow]%d service accounts for all projects[-]", count)
	},
	refresh: func(a *app) {
		a.statusText("[yellow]Fetching service accounts for all projects...[-]")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("accounts", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusText(fmt.Sprintf("[yellow]Fetching service accounts for %s...[-]", p.ProjectID))
				})
				accounts, err := providers.ListProjectServiceAccounts(p.ProjectID)
				if err != nil {
					a.reportError("accounts", p.ProjectID, "Failed to fetch service accounts for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveServiceAccounts(p.ProjectID, accounts); err != nil {
					a.reportError("accounts", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("accounts", p.ProjectID, len(accounts))
				total += len(accounts)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewAccountsAll)
				status := fmt.Sprintf("[green]Fetched %d service accounts across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return serviceAccountResource(a, row[5], row[0])
	},
	samples: []any{providers.ServiceAccount{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPServiceAccountURL(a.info.GetCell(row, 5).Text, a.info.GetCell(row, 0).Text)
	},
}

func showAccounts(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(accountsViewKind())
}
