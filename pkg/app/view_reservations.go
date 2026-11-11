package app

import (
	"fmt"
	"strconv"

	"github.com/elferherrera/lazycloud/cmd/providers"
)

func reservationRows(reservations []providers.Reservation) [][]string {
	rows := make([][]string, 0, len(reservations))
	for _, r := range reservations {
		rows = append(rows, []string{
			r.Name, r.Zone, r.MachineType, strconv.FormatInt(r.Count, 10),
			strconv.FormatInt(r.InUseCount, 10), r.ShareType, r.Status, r.Description,
			formatUpdatedAt(r.UpdatedAt),
		})
	}
	return rows
}

func reservationRowsAll(reservations []providers.Reservation) [][]string {
	rows := reservationRows(reservations)
	for i, r := range reservations {
		rows[i] = append(rows[i], r.ProjectID)
	}
	return rows
}

func reservationResource(a *app, projectID, zone, name string) (any, error) {
	r, err := a.cache.GetReservation(projectID, zone, name)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("reservation not found in cache")
	}
	return r, nil
}

func fetchProjectReservations(a *app, projectID string) (int, string, error) {
	reservations, err := listProjectReservations(projectID)
	if err != nil {
		return 0, "", err
	}
	if err := a.cache.SaveReservations(projectID, reservations); err != nil {
		return 0, "", err
	}
	a.logger.Update("reservations", projectID, len(reservations))
	return len(reservations), fmt.Sprintf("Fetched %d reservations", len(reservations)), nil
}

var reservationsView = viewDef{
	title:   "Reservations",
	headers: []string{"Name", "Zone", "Machine Type", "Reserved", "In Use", "Share", "Status", "Description", "Updated"},
	load: func(a *app) ([][]string, error) {
		reservations, err := a.cache.ListReservations(appState.ProjectID)
		if err != nil {
			return nil, err
		}
		return reservationRows(reservations), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d reservations for %s", count, appState.ProjectID)
	},
	refresh: projectRefresh(viewReservations, "reservations", fetchProjectReservations),
	fetch:   fetchProjectReservations,
	resource: func(a *app, row []string) (any, error) {
		return reservationResource(a, appState.ProjectID, row[1], row[0])
	},
	samples: []any{providers.Reservation{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPReservationsURL(appState.ProjectID, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

var reservationsAllView = viewDef{
	title:   "Reservations",
	headers: []string{"Name", "Zone", "Machine Type", "Reserved", "In Use", "Share", "Status", "Description", "Updated", "Project"},
	load: func(a *app) ([][]string, error) {
		reservations, err := a.cache.ListAllReservations()
		if err != nil {
			return nil, err
		}
		return reservationRowsAll(reservations), nil
	},
	status: func(a *app, count int) string {
		return fmt.Sprintf("%d reservations for all projects", count)
	},
	refresh: allProjectsRefresh(viewReservationsAll, "reservations", "reservations", "reservations", fetchProjectReservations),
	resource: func(a *app, row []string) (any, error) {
		return reservationResource(a, row[9], row[1], row[0])
	},
	samples: []any{providers.Reservation{}},
	consoleURL: func(a *app, row int) string {
		return providers.GCPReservationsURL(a.info.GetCell(row, 9).Text, a.info.GetCell(row, 1).Text, a.info.GetCell(row, 0).Text)
	},
}

func showReservations(a *app) {
	if !appState.AllProjects && appState.ProjectID == "" {
		a.statusText("[red]No project selected[-]")
		return
	}
	a.setView(reservationsViewKind())
}
