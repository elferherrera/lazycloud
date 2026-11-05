package app

import (
	"fmt"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func bucketIAMRows(bindings []providers.BucketIAMBinding) [][]string {
	converted := make([]providers.IAMBinding, 0, len(bindings))
	for _, b := range bindings {
		converted = append(converted, providers.IAMBinding{
			ProjectID: b.ProjectID,
			Member:    b.Member,
			Role:      b.Role,
			Condition: b.Condition,
			UpdatedAt: b.UpdatedAt,
		})
	}
	return iamBindingRows(converted)
}

func bucketIAMResource(a *app, projectID, bucket, member, role string) (any, error) {
	binding, err := a.cache.GetBucketIAMBinding(projectID, bucket, member, role)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, fmt.Errorf("bucket IAM binding not found in cache")
	}
	return binding, nil
}

var bucketIAMView = viewDef{
	title:   "Bucket IAM policy",
	headers: []string{"Member", "Type", "Role", "Condition", "Updated"},
	load: func(a *app) ([][]string, error) {
		bindings, err := a.cache.ListBucketIAMBindings(appState.BucketProject, appState.Bucket)
		if err != nil {
			return nil, err
		}
		return bucketIAMRows(bindings), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d IAM bindings for bucket %s (project %s)", count, appState.Bucket, appState.BucketProject)
	},
	refresh: func(a *app) {
		if appState.BucketProject == "" || appState.Bucket == "" {
			a.statusText("[red]No bucket selected (press i on a bucket row)[-]")
			return
		}
		a.statusInfo(fmt.Sprintf("Fetching IAM policy for %s...", appState.Bucket))
		go func() {
			projectID := appState.BucketProject
			bucket := appState.Bucket
			bindings, err := providers.ListBucketIAMPolicy(projectID, bucket)
			if err != nil {
				a.reportError("bucket-iam", projectID+"/"+bucket, "Failed to fetch bucket IAM policy", err)
				return
			}
			if err := a.cache.SaveBucketIAMBindings(projectID, bucket, bindings); err != nil {
				a.reportError("bucket-iam", projectID+"/"+bucket, "Failed to update cache", err)
				return
			}
			a.logger.Update("bucket-iam", projectID+"/"+bucket, len(bindings))
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewBucketIAM)
				a.statusText(fmt.Sprintf("[green]Fetched %d IAM bindings[-]", len(bindings)))
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return bucketIAMResource(a, appState.BucketProject, appState.Bucket, row[0], row[2])
	},
	samples: []any{providers.BucketIAMBinding{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPBucketIAMURL(appState.BucketProject, appState.Bucket)
	},
}
