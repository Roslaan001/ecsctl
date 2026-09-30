package aws

import (
	"context"
	"errors"
	"reflect"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func TestScaleService(t *testing.T) {
	mock := &mockECS{}
	err := testClient(mock).ScaleService(context.Background(), "cluster-a", "api", 3)
	if err != nil {
		t.Fatalf("ScaleService() error = %v", err)
	}
	if awssdk.ToString(mock.updateServiceInput.Cluster) != "cluster-a" || awssdk.ToString(mock.updateServiceInput.Service) != "api" || awssdk.ToInt32(mock.updateServiceInput.DesiredCount) != 3 {
		t.Fatalf("UpdateService input = %#v", mock.updateServiceInput)
	}
}

func TestDeleteServiceDrainsBeforeDelete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockECS{}
		if err := testClient(mock).DeleteService(context.Background(), "cluster-a", "api"); err != nil {
			t.Fatalf("DeleteService() error = %v", err)
		}
		if awssdk.ToInt32(mock.updateServiceInput.DesiredCount) != 0 {
			t.Fatalf("service was not drained: %#v", mock.updateServiceInput)
		}
		if !mock.deleteServiceCalled || awssdk.ToString(mock.deleteServiceInput.Cluster) != "cluster-a" || awssdk.ToString(mock.deleteServiceInput.Service) != "api" || !awssdk.ToBool(mock.deleteServiceInput.Force) {
			t.Fatalf("DeleteService input = %#v", mock.deleteServiceInput)
		}
	})

	t.Run("drain failure stops before delete", func(t *testing.T) {
		wantErr := errors.New("update failed")
		mock := &mockECS{updateServiceErr: wantErr}
		err := testClient(mock).DeleteService(context.Background(), "cluster-a", "api")
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want wrapped %v", err, wantErr)
		}
		if mock.deleteServiceCalled {
			t.Fatal("DeleteService called after drain failed")
		}
	})

	t.Run("delete failure is returned", func(t *testing.T) {
		wantErr := errors.New("delete failed")
		mock := &mockECS{deleteServiceErr: wantErr}
		err := testClient(mock).DeleteService(context.Background(), "cluster-a", "api")
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
}

func TestRunTaskMapsNetworkOptionsAndReturnsARNs(t *testing.T) {
	mock := &mockECS{runTaskOut: &ecs.RunTaskOutput{Tasks: []types.Task{{TaskArn: awssdk.String("task-arn")}}}}
	got, err := testClient(mock).RunTask(context.Background(), RunTaskOptions{
		Cluster: "cluster-a", TaskDefinition: "worker:3", Count: 2,
		Subnets: []string{"subnet-a", "subnet-b"}, SecurityGroups: []string{"sg-a"}, AssignPublicIP: "ENABLED",
	})
	if err != nil {
		t.Fatalf("RunTask() error = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"task-arn"}) {
		t.Fatalf("task ARNs = %#v", got)
	}
	in := mock.runTaskInput
	if awssdk.ToString(in.Cluster) != "cluster-a" || awssdk.ToString(in.TaskDefinition) != "worker:3" || awssdk.ToInt32(in.Count) != 2 || in.LaunchType != types.LaunchTypeFargate {
		t.Fatalf("RunTask input = %#v", in)
	}
	if in.NetworkConfiguration == nil || in.NetworkConfiguration.AwsvpcConfiguration == nil {
		t.Fatalf("network configuration missing: %#v", in.NetworkConfiguration)
	}
	network := in.NetworkConfiguration.AwsvpcConfiguration
	if !reflect.DeepEqual(network.Subnets, []string{"subnet-a", "subnet-b"}) || !reflect.DeepEqual(network.SecurityGroups, []string{"sg-a"}) || network.AssignPublicIp != types.AssignPublicIpEnabled {
		t.Fatalf("awsvpc configuration = %#v", network)
	}
}

func TestRunTaskReturnsAPIAndTaskFailures(t *testing.T) {
	t.Run("API error", func(t *testing.T) {
		wantErr := errors.New("run failed")
		_, err := testClient(&mockECS{runTaskErr: wantErr}).RunTask(context.Background(), RunTaskOptions{Cluster: "cluster-a"})
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
	})
	t.Run("ECS failure response", func(t *testing.T) {
		mock := &mockECS{runTaskOut: &ecs.RunTaskOutput{Failures: []types.Failure{{Arn: awssdk.String("task-arn"), Reason: awssdk.String("capacity")}}}}
		_, err := testClient(mock).RunTask(context.Background(), RunTaskOptions{Cluster: "cluster-a"})
		if err == nil {
			t.Fatal("expected a task failure error")
		}
		if got := err.Error(); got != "running task failed: task-arn: capacity" {
			t.Fatalf("error = %q", got)
		}
	})
}

func TestStopTaskMapsArguments(t *testing.T) {
	mock := &mockECS{}
	if err := testClient(mock).StopTask(context.Background(), "cluster-a", "task-arn", "cancelled"); err != nil {
		t.Fatalf("StopTask() error = %v", err)
	}
	if awssdk.ToString(mock.stopTaskInput.Cluster) != "cluster-a" || awssdk.ToString(mock.stopTaskInput.Task) != "task-arn" || awssdk.ToString(mock.stopTaskInput.Reason) != "cancelled" {
		t.Fatalf("StopTask input = %#v", mock.stopTaskInput)
	}
}
