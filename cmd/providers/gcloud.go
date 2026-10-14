package providers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	computev1 "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/iterator"
)

type Project struct {
	ProjectID     string
	Name          string
	ProjectNumber string
}

type Network struct {
	ProjectID             string
	Name                  string
	Description           string
	RoutingMode           string
	AutoCreateSubnetworks bool
	MTU                   int32
	UpdatedAt             string
}

type Subnetwork struct {
	ProjectID             string
	Name                  string
	Region                string
	Network               string
	IPRange               string
	SecondaryRanges       []string
	PrivateIPGoogleAccess bool
	Purpose               string
	Description           string
	Status                string
	UpdatedAt             string
}

type IPAddress struct {
	ProjectID   string
	Name        string
	Address     string
	AddressType string
	Status      string
	Region      string
	Subnetwork  string
	Network     string
	UpdatedAt   string
}

type VMInterface struct {
	Network    string
	Subnetwork string
	InternalIP string
	ExternalIP string
}

type VMDisk struct {
	Name       string
	DeviceName string
	Mode       string
	AutoDelete bool
	SizeGB     int64
}

type VM struct {
	ProjectID   string
	Name        string
	InstanceID  string
	Zone        string
	MachineType string
	Status      string
	Interfaces  []VMInterface
	BootDisk    *VMDisk
	DataDisks   []VMDisk
	UpdatedAt   string
}

type Disk struct {
	ProjectID   string
	Name        string
	Zone        string
	SizeGB      int64
	Type        string
	Status      string
	SourceImage string
	Users       []string
	UpdatedAt   string
}

func ListGcloudProjects() ([]Project, error) {
	ctx := context.Background()
	client, err := resourcemanager.NewProjectsClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var projects []Project
	it := client.SearchProjects(ctx, &resourcemanagerpb.SearchProjectsRequest{})
	for {
		p, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		projectNumber := p.GetName()
		if parts := strings.Split(projectNumber, "/"); len(parts) > 1 {
			projectNumber = parts[len(parts)-1]
		}
		projects = append(projects, Project{
			ProjectID:     p.GetProjectId(),
			Name:          p.GetDisplayName(),
			ProjectNumber: projectNumber,
		})
	}
	return projects, nil
}

func resourceShortName(resourceURL, fallback string) string {
	if resourceURL == "" {
		return fallback
	}
	if parts := strings.Split(resourceURL, "/"); len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return resourceURL
}

func ListProjectNetworks(projectID string) ([]Network, error) {
	ctx := context.Background()

	client, err := computev1.NewNetworksRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var networks []Network
	it := client.List(ctx, &computepb.ListNetworksRequest{Project: projectID})
	for {
		n, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		networks = append(networks, Network{
			ProjectID:             projectID,
			Name:                  n.GetName(),
			Description:           n.GetDescription(),
			RoutingMode:           n.GetRoutingConfig().GetRoutingMode(),
			AutoCreateSubnetworks: n.GetAutoCreateSubnetworks(),
			MTU:                   n.GetMtu(),
		})
	}
	return networks, nil
}

func ListProjectSubnetworks(projectID string) ([]Subnetwork, error) {
	ctx := context.Background()

	client, err := computev1.NewSubnetworksRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var subnetworks []Subnetwork
	it := client.AggregatedList(ctx, &computepb.AggregatedListSubnetworksRequest{Project: projectID})
	for {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, sn := range entry.Value.GetSubnetworks() {
			var secondary []string
			for _, r := range sn.GetSecondaryIpRanges() {
				secondary = append(secondary, fmt.Sprintf("%s=%s", r.GetRangeName(), r.GetIpCidrRange()))
			}
			subnetworks = append(subnetworks, Subnetwork{
				ProjectID:             projectID,
				Name:                  sn.GetName(),
				Region:                resourceShortName(sn.GetRegion(), ""),
				Network:               resourceShortName(sn.GetNetwork(), ""),
				IPRange:               sn.GetIpCidrRange(),
				SecondaryRanges:       secondary,
				PrivateIPGoogleAccess: sn.GetPrivateIpGoogleAccess(),
				Purpose:               sn.GetPurpose(),
				Description:           sn.GetDescription(),
				Status:                sn.GetState(),
			})
		}
	}
	return subnetworks, nil
}

type ServiceAccount struct {
	ProjectID   string
	Email       string
	DisplayName string
	UniqueID    string
	Description string
	Disabled    bool
	UpdatedAt   string
}

func ListProjectServiceAccounts(projectID string) ([]ServiceAccount, error) {
	ctx := context.Background()

	svc, err := iam.NewService(ctx)
	if err != nil {
		return nil, err
	}

	var accounts []ServiceAccount
	err = svc.Projects.ServiceAccounts.List("projects/"+projectID).Pages(ctx, func(page *iam.ListServiceAccountsResponse) error {
		for _, sa := range page.Accounts {
			accounts = append(accounts, ServiceAccount{
				ProjectID:   projectID,
				Email:       sa.Email,
				DisplayName: sa.DisplayName,
				UniqueID:    sa.UniqueId,
				Description: sa.Description,
				Disabled:    sa.Disabled,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func vmDisk(d *computepb.AttachedDisk) *VMDisk {
	return &VMDisk{
		Name:       resourceShortName(d.GetSource(), d.GetDeviceName()),
		DeviceName: d.GetDeviceName(),
		Mode:       d.GetMode(),
		AutoDelete: d.GetAutoDelete(),
		SizeGB:     d.GetDiskSizeGb(),
	}
}

func ListProjectVMs(projectID string) ([]VM, error) {
	ctx := context.Background()

	client, err := computev1.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var vms []VM
	it := client.AggregatedList(ctx, &computepb.AggregatedListInstancesRequest{Project: projectID})
	for {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, inst := range entry.Value.GetInstances() {
			if inst.GetZone() == "" {
				continue
			}
			vm := VM{
				ProjectID:   projectID,
				Name:        inst.GetName(),
				InstanceID:  strconv.FormatUint(inst.GetId(), 10),
				Zone:        resourceShortName(inst.GetZone(), ""),
				MachineType: resourceShortName(inst.GetMachineType(), ""),
				Status:      inst.GetStatus(),
			}
			for _, iface := range inst.GetNetworkInterfaces() {
				vmi := VMInterface{
					Network:    resourceShortName(iface.GetNetwork(), ""),
					Subnetwork: resourceShortName(iface.GetSubnetwork(), ""),
					InternalIP: iface.GetNetworkIP(),
				}
				for _, ac := range iface.GetAccessConfigs() {
					if natIP := ac.GetNatIP(); natIP != "" {
						vmi.ExternalIP = natIP
						break
					}
				}
				vm.Interfaces = append(vm.Interfaces, vmi)
			}
			for _, d := range inst.GetDisks() {
				disk := vmDisk(d)
				if d.GetBoot() {
					vm.BootDisk = disk
				} else {
					vm.DataDisks = append(vm.DataDisks, *disk)
				}
			}
			vms = append(vms, vm)
		}
	}
	return vms, nil
}

func ListProjectDisks(projectID string) ([]Disk, error) {
	ctx := context.Background()

	client, err := computev1.NewDisksRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var disks []Disk
	it := client.AggregatedList(ctx, &computepb.AggregatedListDisksRequest{Project: projectID})
	for {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, d := range entry.Value.GetDisks() {
			if d.GetZone() == "" {
				continue
			}
			users := make([]string, 0, len(d.GetUsers()))
			for _, u := range d.GetUsers() {
				users = append(users, resourceShortName(u, ""))
			}
			disks = append(disks, Disk{
				ProjectID:   projectID,
				Name:        d.GetName(),
				Zone:        resourceShortName(d.GetZone(), ""),
				SizeGB:      d.GetSizeGb(),
				Type:        resourceShortName(d.GetType(), ""),
				Status:      d.GetStatus(),
				SourceImage: resourceShortName(d.GetSourceImage(), ""),
				Users:       users,
			})
		}
	}
	return disks, nil
}

func ListProjectIPAddresses(projectID string) ([]IPAddress, error) {
	ctx := context.Background()

	addressesClient, err := computev1.NewAddressesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer addressesClient.Close()

	globalClient, err := computev1.NewGlobalAddressesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	defer globalClient.Close()

	var addresses []IPAddress

	globalIt := globalClient.List(ctx, &computepb.ListGlobalAddressesRequest{Project: projectID})
	for {
		addr, err := globalIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, IPAddress{
			Name:        addr.GetName(),
			Address:     addr.GetAddress(),
			AddressType: addr.GetAddressType(),
			Status:      addr.GetStatus(),
			Region:      "global",
			Subnetwork:  resourceShortName(addr.GetSubnetwork(), ""),
			Network:     resourceShortName(addr.GetNetwork(), ""),
		})
	}

	regionalIt := addressesClient.AggregatedList(ctx, &computepb.AggregatedListAddressesRequest{Project: projectID})
	for {
		entry, err := regionalIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, addr := range entry.Value.GetAddresses() {
			if addr.GetRegion() == "" {
				continue
			}
			region := resourceShortName(addr.GetRegion(), "global")
			addresses = append(addresses, IPAddress{
				Name:        addr.GetName(),
				Address:     addr.GetAddress(),
				AddressType: addr.GetAddressType(),
				Status:      addr.GetStatus(),
				Region:      region,
				Subnetwork:  resourceShortName(addr.GetSubnetwork(), ""),
				Network:     resourceShortName(addr.GetNetwork(), ""),
			})
		}
	}
	return addresses, nil
}
