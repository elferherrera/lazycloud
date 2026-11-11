package app

import (
	"fmt"
	"strconv"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func snapshotRows(snapshots []providers.Snapshot) [][]string {
	rows := make([][]string, 0, len(snapshots))
	for _, s := range snapshots {
		rows = append(rows, []string{
			s.Name, s.SourceDisk, strconv.FormatInt(s.SizeGB, 10), s.Status,
			formatUpdatedAt(s.CreatedAt), s.Description, formatUpdatedAt(s.UpdatedAt),
		})
	}
	return rows
}

func snapshotRowsAll(snapshots []providers.Snapshot) [][]string {
	rows := snapshotRows(snapshots)
	for i, s := range snapshots {
		rows[i] = append(rows[i], s.ProjectID)
	}
	return rows
}

func snapshotResource(a *app, projectID, name string) (any, error) {
	snapshot, err := a.cache.GetSnapshot(projectID, name)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, fmt.Errorf("snapshot not found in cache")
	}
	return snapshot, nil
}

func fetchProjectSnapshots(a *app, projectID string) (int, string, error) {
	snapshots, err := listProjectSnapshots(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveSnapshots(projectID, snapshots); err != nil {
		return 0, "", err
	}
	a.logger.Update("snapshots", projectID, len(snapshots))
	return len(snapshots), fmt.Sprintf("Fetched %d snapshots", len(snapshots)), nil
}

var snapshotsView = viewDef{
	title:   "Snapshots",
	headers: []string{"Name", "Disk", "Size GB", "Status", "Created", "Description", "Updated"},
	load: func(a *app) ([][]string, error) {
		snapshots, err := a.cache.ListSnapshots(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return snapshotRows(snapshots), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d snapshots for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewSnapshots, "snapshots", fetchProjectSnapshots),
	fetch:   fetchProjectSnapshots,
	resource: func(a *app, row []string) (any, error) {
		return snapshotResource(a, appState.ProjectID, row[0])
	},
	samples: []any{providers.Snapshot{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPSnapshotURL(appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
}

var snapshotsAllView = viewDef{
	title:   "Snapshots",
	headers: []string{"Name", "Disk", "Size GB", "Status", "Created", "Description", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		snapshots, err := a.cache.ListAllSnapshots()
		if err != nil {
			return nil, err
		}
		return snapshotRowsAll(snapshots), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d snapshots for all projects", count)
	},
	refresh: allProjectsRefresh(viewSnapshotsAll, "snapshots", "snapshots", "snapshots", fetchProjectSnapshots),
	resource: func(a *app, row []string) (any, error) {
		return snapshotResource(a, row[7], row[0])
	},
	samples: []any{providers.Snapshot{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPSnapshotURL(a.info.GetCell(row, 7).Text, a.info.GetCell(row, 0).Text)
	},
}

func showSnapshots(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(snapshotsViewKind())
}
