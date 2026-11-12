package app

import (
	"fmt"

	"github.com/elferherrera/lazycloud/pkg/providers"
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

func fetchProjectAccounts(a *app, projectID string) (int, string, error) {
	accounts, err := listProjectServiceAccounts(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveServiceAccounts(projectID, accounts); err != nil {
		return 0, "", err
	}
	a.logger.Update("accounts", projectID, len(accounts))
	return len(accounts), fmt.Sprintf("Fetched %d service accounts", len(accounts)), nil
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
		return fmt.Sprintf("%d service accounts for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewAccounts, "service accounts", fetchProjectAccounts),
	fetch:   fetchProjectAccounts,
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
		return fmt.Sprintf("%d service accounts for all projects", count)
	},
	refresh: allProjectsRefresh(viewAccountsAll, "accounts", "service accounts", "service accounts", fetchProjectAccounts),
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
