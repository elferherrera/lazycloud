package app

import (
	"fmt"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func bucketRows(buckets []providers.Bucket) [][]string {
	rows := make([][]string, 0, len(buckets))
	for _, b := range buckets {
		rows = append(rows, []string{
			b.Name, b.Location, b.StorageClass, yesNo(b.Versioning), formatUpdatedAt(b.CreatedAt), formatUpdatedAt(b.UpdatedAt),
		})
	}
	return rows
}

func bucketRowsAll(buckets []providers.Bucket) [][]string {
	rows := bucketRows(buckets)
	for i, b := range buckets {
		rows[i] = append(rows[i], b.ProjectID)
	}
	return rows
}

func bucketResource(a *app, projectID, name string) (any, error) {
	b, err := a.cache.GetBucket(projectID, name)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, fmt.Errorf("bucket not found in cache")
	}
	return b, nil
}

var bucketsView = viewDef{
	title:   "Buckets",
	headers: []string{"Name", "Location", "Storage Class", "Versioning", "Created", "Updated"},
	load: func(a *app) ([][]string, error) {
		buckets, err := a.cache.ListBuckets(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return bucketRows(buckets), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d buckets for %s", count, appState.ProjectID)
	},
	refresh: func(a *app) {
		if appState.ProjectID == "" {
			a.statusText("[red]No project selected[-]")
			return
		}
		a.statusInfo("Fetching buckets...")
		go func() {
			projectID := appState.ProjectID
			buckets, err := providers.ListProjectBuckets(projectID)
			if err != nil {
				a.reportError("buckets", projectID, "Failed to fetch buckets", err)
				return
			}
			if err := a.cache.SaveBuckets(projectID, buckets); err != nil {
				a.reportError("buckets", projectID, "Failed to update cache", err)
				return
			}
			a.logger.Update("buckets", projectID, len(buckets))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewBuckets)
				a.statusText(fmt.Sprintf("[green]Fetched %d buckets[-]", len(buckets)))
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return bucketResource(a, appState.ProjectID, row[0])
	},
	samples: []any{providers.Bucket{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPBucketURL(appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
}

var bucketsAllView = viewDef{
	title:   "Buckets",
	headers: []string{"Name", "Location", "Storage Class", "Versioning", "Created", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		buckets, err := a.cache.ListAllBuckets()
		if err != nil {
			return nil, err
		}
		return bucketRowsAll(buckets), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d buckets for all projects", count)
	},
	refresh: func(a *app) {
		a.statusInfo("Fetching buckets for all projects...")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("buckets", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusInfo(fmt.Sprintf("Fetching buckets for %s...", p.ProjectID))
				})
				buckets, err := providers.ListProjectBuckets(p.ProjectID)
				if err != nil {
					a.reportError("buckets", p.ProjectID, "Failed to fetch buckets for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveBuckets(p.ProjectID, buckets); err != nil {
					a.reportError("buckets", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("buckets", p.ProjectID, len(buckets))
				total += len(buckets)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewBucketsAll)
				status := fmt.Sprintf("[green]Fetched %d buckets across %d projects[-]", total, len(projects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return bucketResource(a, row[6], row[0])
	},
	samples: []any{providers.Bucket{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPBucketURL(a.info.GetCell(row, 6).Text, a.info.GetCell(row, 0).Text)
	},
}

func showBuckets(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(bucketsViewKind())
}
