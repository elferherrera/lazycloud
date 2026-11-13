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

func fetchProjectBuckets(a *app, projectID string) (int, string, error) {
	buckets, err := listProjectBuckets(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveBuckets(projectID, buckets); err != nil {
		return 0, "", err
	}
	a.logger.Update("buckets", projectID, len(buckets))
	return len(buckets), fmt.Sprintf("Fetched %d buckets", len(buckets)), nil
}

func (a *app) showBucketIAM() {
	row, _ := a.info.GetSelection()
	if row <= 0 || row >= a.info.GetRowCount() {
		return
	}
	projectID := appState.ProjectID
	if appState.ViewKind == viewBucketsAll {
		projectID = a.info.GetCell(row, 6).Text
	}
	appState.BucketProject = projectID
	appState.Bucket = a.info.GetCell(row, 0).Text
	a.setView(viewBucketIAM)
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
	refresh: projectRefresh(viewBuckets, "buckets", fetchProjectBuckets),
	fetch:   fetchProjectBuckets,
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
	refresh: allProjectsRefresh(viewBucketsAll, "buckets", "buckets", "buckets", fetchProjectBuckets),
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
