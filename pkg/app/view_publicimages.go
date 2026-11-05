package app

import (
	"fmt"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

var publicImagesView = viewDef{
	title:   "Public images",
	headers: []string{"Name", "Family", "Status", "Size GB", "Location", "Source", "Created", "Description", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		images, err := a.cache.ListPublicImages()
		if err != nil {
			return nil, err
		}
		return imageRowsAll(images), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d public images", count)
	},
	refresh: func(a *app) {
		a.statusInfo("Fetching public images...")
		go func() {
			total := 0
			failed := 0
			for _, projectID := range providers.PublicImageProjects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusInfo(fmt.Sprintf("Fetching public images from %s...", projectID))
				})
				images, err := providers.ListProjectImages(projectID)
				if err != nil {
					a.reportError("public-images", projectID, "Failed to fetch public images from "+projectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveImages(projectID, images); err != nil {
					a.reportError("public-images", projectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("public-images", projectID, len(images))
				total += len(images)
			}
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewPublicImages)
				status := fmt.Sprintf("[green]Fetched %d public images across %d projects[-]", total, len(providers.PublicImageProjects))
				if failed > 0 {
					status += fmt.Sprintf(" [red](%d projects failed, see log)[-]", failed)
				}
				a.statusText(status)
			})
		}()
	},
	resource: func(a *app, row []string) (any, error) {
		return imageResource(a, row[9], row[0])
	},
	samples: []any{providers.Image{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPImageURL(a.info.GetCell(row, 9).Text, a.info.GetCell(row, 0).Text)
	},
}

func showPublicImages(a *app) {
	a.setView(viewPublicImages)
}
