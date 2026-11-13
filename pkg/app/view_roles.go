package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func customRoleRows(roles []providers.CustomRole) [][]string {
	rows := make([][]string, 0, len(roles))
	for _, r := range roles {
		rows = append(rows, []string{
			r.Name, r.Title, r.Stage, strconv.Itoa(len(r.Permissions)),
			r.Description, formatUpdatedAt(r.UpdatedAt),
		})
	}
	return rows
}

func customRoleRowsAll(roles []providers.CustomRole) [][]string {
	rows := customRoleRows(roles)
	for i, r := range roles {
		rows[i] = append(rows[i], r.ProjectID)
	}
	return rows
}

func customRoleResource(a *app, projectID, name string) (any, error) {
	role, err := a.cache.GetCustomRole(projectID, name)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, fmt.Errorf("custom role not found in cache")
	}
	return role, nil
}

func fetchProjectRoles(a *app, projectID string) (int, string, error) {
	roles, err := listProjectCustomRoles(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveCustomRoles(projectID, roles); err != nil {
		return 0, "", err
	}
	a.logger.Update("roles", projectID, len(roles))
	return len(roles), fmt.Sprintf("Fetched %d custom roles", len(roles)), nil
}

func customRoleDetails(res any) string {
	role := res.(*providers.CustomRole)
	var sb strings.Builder
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Project: [-]%s\n", role.ProjectID)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Name: [-]%s\n", role.Name)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Title: [-]%s\n", role.Title)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Stage: [-]%s\n", role.Stage)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Deleted: [-]%v\n", role.Deleted)
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Description: [-]%s\n", role.Description)
	sb.WriteString(theme.detailLabelTag() + "Permissions: [-]\n")
	for _, p := range role.Permissions {
		fmt.Fprintf(&sb, "  %s\n", p)
	}
	fmt.Fprintf(&sb, theme.detailLabelTag()+"Updated: [-]%s\n", formatUpdatedAt(role.UpdatedAt))
	return sb.String()
}

var rolesView = viewDef{
	title:   "Custom roles",
	headers: []string{"Name", "Title", "Stage", "Permissions", "Description", "Updated"},
	load: func(a *app) ([][]string, error) {
		roles, err := a.cache.ListCustomRoles(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return customRoleRows(roles), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d custom roles for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewRoles, "custom roles", fetchProjectRoles),
	fetch:   fetchProjectRoles,
	resource: func(a *app, row []string) (any, error) {
		return customRoleResource(a, appState.ProjectID, row[0])
	},
	details: customRoleDetails,
	samples: []any{providers.CustomRole{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPCustomRoleURL(appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
}

var rolesAllView = viewDef{
	title:   "Custom roles",
	headers: []string{"Name", "Title", "Stage", "Permissions", "Description", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		roles, err := a.cache.ListAllCustomRoles()
		if err != nil {
			return nil, err
		}
		return customRoleRowsAll(roles), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d custom roles for all projects", count)
	},
	refresh: func(a *app) {
		a.statusInfo("Fetching custom roles for all projects...")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("roles", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusInfo(fmt.Sprintf("Fetching custom roles for %s...", p.ProjectID))
				})
				roles, err := listProjectCustomRoles(p.ProjectID)
				if err != nil {
					a.reportError("roles", p.ProjectID, "Failed to fetch custom roles for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveCustomRoles(p.ProjectID, roles); err != nil {
					a.reportError("roles", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("roles", p.ProjectID, len(roles))
				total += len(roles)
			}
			a.markViewUpdated(viewRolesAll, appState.ProjectID)
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewRolesAll)
				status := fmt.Sprintf("[green]Fetched %d custom roles across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return customRoleResource(a, row[6], row[0])
	},
	details: customRoleDetails,
	samples: []any{providers.CustomRole{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPCustomRoleURL(a.info.GetCell(row, 6).Text, a.info.GetCell(row, 0).Text)
	},
}

func showRoles(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(rolesViewKind())
}