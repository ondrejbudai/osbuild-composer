// +build integration

package openstacktest

import (
	"fmt"
	"os"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imagedata"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
)

const WaitTimeout = 30 * 60 // 30 minutes in seconds

func UploadImageToOpenStack(p *gophercloud.ProviderClient, imagePath string, imageName string) (*images.Image, error) {
	client, err := openstack.NewImageServiceV2(p, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating ImageService client: %v", err)
	}

	// create a new image which gives us the ID
	image, err := images.Create(client, images.CreateOpts{
		Name:            imageName,
		DiskFormat:      "qcow2",
		ContainerFormat: "bare",
	}).Extract()
	if err != nil {
		return image, fmt.Errorf("Creating image failed: %v", err)
	}

	// then upload the actual binary data
	imageData, err := os.Open(imagePath)
	if err != nil {
		return image, fmt.Errorf("Error opening %s: %v", imagePath, err)
	}
	defer imageData.Close()

	err = imagedata.Upload(client, image.ID, imageData).ExtractErr()
	if err != nil {
		return image, fmt.Errorf("Upload to OpenStack failed: %v", err)
	}

	// wait for the status to change from Queued to Active
	err = gophercloud.WaitFor(WaitTimeout, func() (bool, error) {
		actual, err := images.Get(client, image.ID).Extract()
		return actual.Status == images.ImageStatusActive, err
	})
	if err != nil {
		return image, fmt.Errorf("Waiting for image to become Active failed: %v", err)
	}

	return image, nil
}

func DeleteImageFromOpenStack(p *gophercloud.ProviderClient, imageUUID string) error {
	client, err := openstack.NewImageServiceV2(p, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		return fmt.Errorf("Error creating ImageService client: %v", err)
	}

	err = images.Delete(client, imageUUID).ExtractErr()
	if err != nil {
		return fmt.Errorf("cannot delete the image: %v", err)
	}

	return nil
}

func WithBootedImageInOpenStack(p *gophercloud.ProviderClient, imageID, userData string, f func(address string) error) (retErr error) {
	client, err := openstack.NewComputeV2(p, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		return fmt.Errorf("Error creating Compute client: %v", err)
	}

	server, err := servers.Create(client, servers.CreateOpts{
		Name:      "osbuild-composer-vm-for-" + imageID,
		FlavorRef: "ca2a6e9c-2236-4107-8905-7ae9427132ff", // v1-standard-2
		Networks: []servers.Network{ // public
			servers.Network{UUID: "6d6357ac-0f70-4afa-8bd7-c274cc4ea235"},
		},
		ImageRef: imageID,
		UserData: []byte(userData),
	}).Extract()
	if err != nil {
		return fmt.Errorf("Cannot create instance: %v", err)
	}

	// cleanup
	defer func() {
		err := servers.ForceDelete(client, server.ID).ExtractErr()
		if err != nil {
			fmt.Printf("Force deleting instance %s failed: %v", server.ID, err)
			return
		}
	}()

	// wait for the status to become Active
	err = servers.WaitForStatus(client, server.ID, "ACTIVE", WaitTimeout)
	if err != nil {
		return fmt.Errorf("Waiting for instance %s to become Active failed: %v", server.ID, err)
	}

	// get server details again to refresh the IP addresses
	server, err = servers.Get(client, server.ID).Extract()
	if err != nil {
		return fmt.Errorf("Cannot get instance details: %v\n", err)
	}

	// server.AccessIPv4 is empty so list all addresses and
	// get the first fixed one. ssh should be equally happy with v4 or v6
	var fixedIP string
	for _, networkAddresses := range server.Addresses["provider_net_cci_2"].([]interface{}) {
		address := networkAddresses.(map[string]interface{})
		if address["OS-EXT-IPS:type"] == "fixed" {
			fixedIP = address["addr"].(string)
			break
		}
	}
	if fixedIP == "" {
		return fmt.Errorf("Cannot find IP address for instance %s", server.ID)
	}

	return f(fixedIP)
}
