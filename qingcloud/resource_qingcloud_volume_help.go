package qingcloud

import (
	"errors"
	"fmt"

	"github.com/hashicorp/terraform/helper/schema"
	qc "github.com/yunify/qingcloud-sdk-go/service"
)

func motifyVolumeAttributes(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).volume
	input := new(qc.ModifyVolumeAttributesInput)
	input.Volume = qc.String(d.Id())

	attributeUpdate := false
	if d.HasChange("description") {
		if d.Get("description").(string) != "" {
			input.Description = qc.String(d.Get("description").(string))
		} else {
			input.Description = qc.String(" ")
		}
		attributeUpdate = true
	}
	if d.HasChange("name") && !d.IsNewResource() {
		if d.Get("name").(string) != "" {
			input.VolumeName = qc.String(d.Get("name").(string))
		} else {
			input.VolumeName = qc.String(" ")
		}
		attributeUpdate = true
	}
	if attributeUpdate {
		var err error
		simpleRetry(func() error {
			_, err := clt.ModifyVolumeAttributes(input)
			return isServerBusy(err)
		})
		return err
	}
	return nil
}

func changeVolumeSize(d *schema.ResourceData, meta interface{}) error {
	if !d.HasChange("size") || d.IsNewResource() {
		return nil
	}
	clt := meta.(*QingCloudClient).volume
	// new size must bigger than old size
	oldV, newV := d.GetChange("size")
	oldSize := oldV.(int)
	newSize := newV.(int)
	if oldSize >= newSize {
		return errors.New("volume size can't reduce")
	}
	describeInput := new(qc.DescribeVolumesInput)
	describeInput.Volumes = []*string{qc.String(d.Id())}
	var describeOutput *qc.DescribeVolumesOutput
	var err error
	simpleRetry(func() error {
		describeOutput, err = clt.DescribeVolumes(describeInput)
		return isServerBusy(err)
	})
	if err != nil {
		return err
	}
	if qc.StringValue(describeOutput.VolumeSet[0].Status) != "available" {
		return fmt.Errorf("Only when the state of the volume is available can it be expanded ")
	}
	// increase disk size
	input := new(qc.ResizeVolumesInput)
	input.Volumes = []*string{qc.String(d.Id())}
	input.Size = qc.Int(newSize)
	simpleRetry(func() error {
		_, err = clt.ResizeVolumes(input)
		return isServerBusy(err)
	})
	if err != nil {
		return err
	}
	if _, err := VolumeTransitionStateRefresh(clt, d.Id()); err != nil {
		return err
	}
	return nil
}

func waitVolumeLease(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).volume
	input := new(qc.DescribeVolumesInput)
	input.Volumes = []*string{qc.String(d.Id())}
	input.Verbose = qc.Int(1)
	var output *qc.DescribeVolumesOutput
	var err error
	simpleRetry(func() error {
		output, err = clt.DescribeVolumes(input)
		return isServerBusy(err)
	})
	if err != nil {
		return err
	}
	//wait for lease info
	WaitForLease(output.VolumeSet[0].CreateTime)
	return nil
}
