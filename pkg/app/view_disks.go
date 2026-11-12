package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/elferherrera/lazycloud/pkg/providers"
)

func diskRows(disks []providers.Disk) [][]string {
	rows := make([][]string, 0, len(disks))
	for _, d := range disks {
		rows = append(rows, []string{
			d.Name, d.Zone, strconv.FormatInt(d.SizeGB, 10), d.Type, d.Status,
			strings.Join(d.Users, ","), d.SourceImage, formatUpdatedAt(d.UpdatedAt),
		})
	}
	return rows
}

func diskRowsAll(disks []providers.Disk) [][]string {
	rows := diskRows(disks)
	for i, d := range disks {
		rows[i] = append(rows[i], d.ProjectID)
	}
	return rows
}

func diskResource(a *app, projectID, zone, name string) (any, error) {
	disk, err := a.cache.GetDisk(projectID, zone, name)
	if err != nil {
		return nil, err
	}
	if disk == nil {
		return nil, fmt.Errorf("disk not found in cache")
	}
	return disk, nil
}

func fetchProjectDisks(a *app, projectID string) (int, string, error) {
	disks, err := listProjectDisks(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveDisks(projectID, disks); err != nil {
		return 0, "", err
	}
	a.logger.Update("disks", projectID, len(disks))
	return len(disks), fmt.Sprintf("Fetched %d disks", len(disks)), nil
}

var disksView = viewDef{
	title:   "Disks",
	headers: []string{"Name", "Zone", "Size GB", "Type", "Status", "In Use By", "Source Image", "Updated"},
	load: func(a *app) ([][]string, error) {
		disks, err := a.cache.ListDisks(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return diskRows(disks), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d disks for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewDisks, "disks", fetchProjectDisks),
	fetch:   fetchProjectDisks,
	resource: func(a *app, row []string) (any, error) {
		return diskResource(a, appState.ProjectID, row[1], row[0])
	},
	samples: []any{providers.Disk{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPDiskURL(appState.ProjectID, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

var disksAllView = viewDef{
	title:   "Disks",
	headers: []string{"Name", "Zone", "Size GB", "Type", "Status", "In Use By", "Source Image", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		disks, err := a.cache.ListAllDisks()
		if err != nil {
			return nil, err
		}
		return diskRowsAll(disks), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d disks for all projects", count)
	},
	refresh: allProjectsRefresh(viewDisksAll, "disks", "disks", "disks", fetchProjectDisks),
	resource: func(a *app, row []string) (any, error) {
		return diskResource(a, row[8], row[1], row[0])
	},
	samples: []any{providers.Disk{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPDiskURL(a.info.GetCell(row, 8).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

func showDisks(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(disksViewKind())
}
