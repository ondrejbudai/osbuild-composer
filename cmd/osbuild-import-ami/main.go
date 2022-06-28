package main

import (
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func main() {
	image := flag.String("image", "", "image")
	arch := flag.String("arch", "", "arch")
	name := flag.String("name", "", "name")

	flag.Parse()

	// Create a Session with a custom region
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	})
	if err != nil {
		panic(err)
	}

	c := ec2.New(sess)

	identity, err := ec2metadata.New(sess).GetInstanceIdentityDocument()
	if err != nil {
		panic(err)
	}
	az := identity.AvailabilityZone
	//az := "us-east-1a"
	instanceID := identity.InstanceID

	v, err := c.CreateVolume(&ec2.CreateVolumeInput{
		AvailabilityZone: aws.String(az),
		Size:             aws.Int64(10),
	})
	if err != nil {
		panic(err)
	}

	for {
		v2, err := c.DescribeVolumes(&ec2.DescribeVolumesInput{
			VolumeIds: []*string{v.VolumeId},
		})
		if err != nil {
			panic(err)
		}

		state := *v2.Volumes[0].State
		if state == "creating" {
			log.Print("creating")
			time.Sleep(time.Second)
			continue
		}

		if state == "available" {
			log.Print("done")
			break
		}

		panic("invalid state")
	}

	log.Print(*v.VolumeId)

	dev := "/dev/sdz"
	_, err = c.AttachVolume(&ec2.AttachVolumeInput{
		Device:     aws.String(dev),
		InstanceId: aws.String(instanceID),
		VolumeId:   v.VolumeId,
	})

	if err != nil {
		panic(err)
	}

	for {
		v2, err := c.DescribeVolumes(&ec2.DescribeVolumesInput{
			VolumeIds: []*string{v.VolumeId},
		})
		if err != nil {
			panic(err)
		}

		state := *v2.Volumes[0].Attachments[0].State
		if state == "attaching" {
			log.Print("attaching")
			time.Sleep(time.Second)
			continue
		}

		if state == "attached" {
			log.Print("attached")
			break
		}

		panic("invalid state")
	}

	for {
		_, err = os.Stat("/dev/nvme1n1")

		if errors.Is(err, os.ErrNotExist) {
			log.Print("waiting")
			time.Sleep(time.Second)
			continue
		}

		if err != nil {
			panic(err)
		}

		break
	}

	log.Print("opening image")
	in, err := os.Open(*image)
	if err != nil {
		panic(err)
	}

	defer in.Close()

	log.Print("opening drive")

	out, err := os.OpenFile("/dev/nvme1n1", os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}

	_, err = io.Copy(out, in)
	out.Close()
	if err != nil {
		panic(err)
	}

	_, err = c.DetachVolume(&ec2.DetachVolumeInput{
		VolumeId: v.VolumeId,
	})
	if err != nil {
		panic(err)
	}

	for {
		v2, err := c.DescribeVolumes(&ec2.DescribeVolumesInput{
			VolumeIds: []*string{v.VolumeId},
		})
		if err != nil {
			panic(err)
		}

		if *v2.Volumes[0].State == "available" {
			break
		}

		aState := *v2.Volumes[0].Attachments[0].State
		if aState == "detaching" {
			log.Print("detaching")
			time.Sleep(time.Second)
			continue
		}

		panic("invalid state")
	}

	s, err := c.CreateSnapshot(&ec2.CreateSnapshotInput{
		VolumeId: v.VolumeId,
	})

	if err != nil {
		panic(err)
	}

	for {
		so, err := c.DescribeSnapshots(&ec2.DescribeSnapshotsInput{
			SnapshotIds: []*string{s.SnapshotId},
		})
		if err != nil {
			panic(err)
		}

		state := *so.Snapshots[0].State
		if state == "pending" {
			log.Print("pending")
			time.Sleep(time.Second)
			continue
		}

		if state == "completed" {
			log.Print("completed")
			break
		}

		panic("invalid state " + state)
	}

	r, err := c.RegisterImage(
		&ec2.RegisterImageInput{
			Architecture:       arch,
			VirtualizationType: aws.String("hvm"),
			Name:               name,
			RootDeviceName:     aws.String("/dev/sda1"),
			EnaSupport:         aws.Bool(true),
			BlockDeviceMappings: []*ec2.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/sda1"),
					Ebs: &ec2.EbsBlockDevice{
						SnapshotId: s.SnapshotId,
					},
				},
			},
		},
	)

	if err != nil {
		panic(err)
	}

	log.Print(*r.ImageId)

	_, err = c.DeleteVolume(&ec2.DeleteVolumeInput{
		VolumeId: v.VolumeId,
	})
	if err != nil {
		panic(err)
	}
}
