package app

import (
	"fmt"
	"strconv"

	"github.com/elferherrera/lazycloud/pkg/providers"
	"github.com/elferherrera/lazycloud/pkg/providers/gcp"
)

const (
	pscKindAttachment = "attachment"
	pscKindEndpoint   = "endpoint"
)

func pscEndpoints(rules []providers.ForwardingRule) []providers.ForwardingRule {
	var endpoints []providers.ForwardingRule
	for _, r := range rules {
		if r.TargetKind == "serviceAttachments" {
			endpoints = append(endpoints, r)
		}
	}
	return endpoints
}

func pscEndpointTargetProject(r providers.ForwardingRule) string {
	if r.TargetURL == "" {
		return ""
	}
	return gcp.ResourceProjectName(r.TargetURL)
}

func pscEndpointTargetRef(r providers.ForwardingRule) string {
	if r.TargetURL != "" {
		return gcp.ResourcePath(r.TargetURL)
	}
	return r.Target
}

func pscRows(attachments []providers.ServiceAttachment, endpoints []providers.ForwardingRule) [][]string {
	rows := make([][]string, 0, len(attachments)+len(endpoints))
	for _, sa := range attachments {
		rows = append(rows, []string{
			pscKindAttachment, sa.Name, sa.Region, sa.TargetService, "", "", strconv.Itoa(len(sa.Connections)), sa.Description, formatUpdatedAt(sa.UpdatedAt),
		})
	}
	for _, r := range endpoints {
		rows = append(rows, []string{
			pscKindEndpoint, r.Name, r.Region, r.Target, pscEndpointTargetProject(r), r.IPAddress, "", pscEndpointTargetRef(r), formatUpdatedAt(r.UpdatedAt),
		})
	}
	return rows
}

func pscRowsAll(attachments []providers.ServiceAttachment, endpoints []providers.ForwardingRule) [][]string {
	rows := pscRows(attachments, endpoints)
	for i, sa := range attachments {
		rows[i] = append(rows[i], sa.ProjectID)
	}
	for i, r := range endpoints {
		rows[len(attachments)+i] = append(rows[len(attachments)+i], r.ProjectID)
	}
	return rows
}

func pscResource(a *app, projectID, kind, region, name string) (any, error) {
	switch kind {
	case pscKindAttachment:
		sa, err := a.cache.GetServiceAttachment(projectID, region, name)
		if err != nil {
			return nil, err
		}
		if sa == nil {
			return nil, fmt.Errorf("service attachment not found in cache")
		}
		return sa, nil
	case pscKindEndpoint:
		r, err := a.cache.GetForwardingRule(projectID, region, name)
		if err != nil {
			return nil, err
		}
		if r == nil {
			return nil, fmt.Errorf("forwarding rule not found in cache")
		}
		return r, nil
	default:
		return nil, fmt.Errorf("unknown PSC kind %q", kind)
	}
}

func fetchProjectPSC(a *app, projectID string) (int, string, error) {
	attachments, err := listProjectServiceAttach(projectID)
	if err != nil {
		return 0, "", err
	}
	rules, err := listProjectForwardingRules(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveServiceAttachments(projectID, attachments); err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveForwardingRules(projectID, rules); err != nil {
		return 0, "", err
	}
	endpoints := pscEndpoints(rules)
	total := len(attachments) + len(endpoints)
	a.logger.Update("psc", projectID, total)
	return total, fmt.Sprintf("Fetched %d PSC attachments and %d endpoints", len(attachments), len(endpoints)), nil
}

var pscView = viewDef{
	title:   "Private Service Connect",
	headers: []string{"Type", "Name", "Region", "Target", "Target Project", "IP", "Connections", "Description", "Updated"},
	load: func(a *app) ([][]string, error) {
		attachments, err := a.cache.ListServiceAttachments(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		rules, err := a.cache.ListForwardingRules(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return pscRows(attachments, pscEndpoints(rules)), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d PSC resources for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewPSC, "PSC resources", fetchProjectPSC),
	fetch:   fetchProjectPSC,
	resource: func(a *app, row []string) (any, error) {
		return pscResource(a, appState.ProjectID, row[0], row[2], row[1])
	},
	samples: []any{providers.ServiceAttachment{}, providers.ForwardingRule{}},
	consoleURL: func(a *app, row int) string {
		if a.info.GetCell(row, 0).Text == pscKindAttachment {
			return providers.GCPPSCAttachmentURL(appState.ProjectID, a.info.GetCell(row, 2).Text, a.info.GetCell(row, 1).Text)
		}
		return providers.GCPPSCEndpointURL(appState.ProjectID, a.info.GetCell(row, 2).Text, a.info.GetCell(row, 1).Text)
	},
}

var pscAllView = viewDef{
	title:   "Private Service Connect",
	headers: []string{"Type", "Name", "Region", "Target", "Target Project", "IP", "Connections", "Description", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		attachments, err := a.cache.ListAllServiceAttachments()
		if err != nil {
			return nil, err
		}
		rules, err := a.cache.ListAllForwardingRules()
		if err != nil {
			return nil, err
		}
		return pscRowsAll(attachments, pscEndpoints(rules)), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d PSC resources for all projects", count)
	},
	refresh: allProjectsRefresh(viewPSCAll, "psc", "PSC resources", "PSC resources", fetchProjectPSC),
	resource: func(a *app, row []string) (any, error) {
		return pscResource(a, row[9], row[0], row[2], row[1])
	},
	samples: []any{providers.ServiceAttachment{}, providers.ForwardingRule{}},
	consoleURL: func(a *app, row int) string {
		projectID := a.info.GetCell(row, 9).Text
		region := a.info.GetCell(row, 2).Text
		name := a.info.GetCell(row, 1).Text
		if a.info.GetCell(row, 0).Text == pscKindAttachment {
			return providers.GCPPSCAttachmentURL(projectID, region, name)
		}
		return providers.GCPPSCEndpointURL(projectID, region, name)
	},
}

func showPSC(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(pscViewKind())
}
