package app

import (
	"fmt"
	"strconv"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func imageRows(images []providers.Image) [][]string {
	rows := make([][]string, 0, len(images))
	for _, i := range images {
		rows = append(rows, []string{
			i.Name, i.Family, i.Status, strconv.FormatInt(i.SizeGB, 10),
			i.StorageLocation, i.Source, formatUpdatedAt(i.CreatedAt), i.Description,
			formatUpdatedAt(i.UpdatedAt),
		})
	}
	return rows
}

func imageRowsAll(images []providers.Image) [][]string {
	rows := imageRows(images)
	for i, img := range images {
		rows[i] = append(rows[i], img.ProjectID)
	}
	return rows
}

func imageResource(a *app, projectID, name string) (any, error) {
	img, err := a.cache.GetImage(projectID, name)
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, fmt.Errorf("image not found in cache")
	}
	return img, nil
}

func fetchProjectImages(a *app, projectID string) (string, error) {
	images, err := providers.ListProjectImages(projectID)
	if err != nil {
		return "", err
	}
	if err := a.cache.SaveImages(projectID, images); err != nil {
		return "", err
	}
	a.logger.Update("images", projectID, len(images))
	return fmt.Sprintf("Fetched %d images", len(images)), nil
}

var imagesView = viewDef{
	title:   "Images",
	headers: []string{"Name", "Family", "Status", "Size GB", "Location", "Source", "Created", "Description", "Updated"},
	load: func(a *app) ([][]string, error) {
		images, err := a.cache.ListImages(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return imageRows(images), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d images for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewImages, "images", fetchProjectImages),
	fetch:   fetchProjectImages,
	resource: func(a *app, row []string) (any, error) {
		return imageResource(a, appState.ProjectID, row[0])
	},
	samples: []any{providers.Image{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPImageURL(appState.ProjectID, a.info.GetCell(row, 0).Text)
	},
}

var imagesAllView = viewDef{
	title:   "Images",
	headers: []string{"Name", "Family", "Status", "Size GB", "Location", "Source", "Created", "Description", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		images, err := a.cache.ListAllImages()
		if err != nil {
			return nil, err
		}
		return imageRowsAll(images), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d images for all projects", count)
	},
	refresh: func(a *app) {
		a.statusInfo("Fetching images for all projects...")
		go func() {
			projects, err := a.cache.ListProjects()
			if err != nil {
				a.reportError("images", "all", "Failed to list projects", err)
				return
			}
			total := 0
			failed := 0
			for _, p := range projects {
				a.tapp.QueueUpdateDraw(func() {
					a.statusInfo(fmt.Sprintf("Fetching images for %s...", p.ProjectID))
				})
				images, err := providers.ListProjectImages(p.ProjectID)
				if err != nil {
					a.reportError("images", p.ProjectID, "Failed to fetch images for "+p.ProjectID, err)
					failed++
					continue
				}
				if err := a.cache.SaveImages(p.ProjectID, images); err != nil {
					a.reportError("images", p.ProjectID, "Failed to update cache", err)
					failed++
					continue
				}
				a.logger.Update("images", p.ProjectID, len(images))
				total += len(images)
			}
			a.markViewUpdated(viewImagesAll, appState.ProjectID)
			a.tapp.QueueUpdateDraw(func() {
				a.setView(viewImagesAll)
				status := fmt.Sprintf("[green]Fetched %d images across %d projects[-]", total, len(projects))
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

func showImages(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(imagesViewKind())
}
