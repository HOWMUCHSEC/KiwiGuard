package application

import (
	"context"
	"reflect"
	"testing"
)

func TestExchangeWorkflowUseCaseRunsSuccessfulExchange(t *testing.T) {
	driver := &recordingExchangeWorkflowDriver{
		prepareOK: true,
		inputOK:   true,
		projectOK: true,
	}

	completed := (ExchangeWorkflowUseCase{}).Run(context.Background(), driver)

	if !completed {
		t.Fatal("Run() = false, want true")
	}
	want := []string{"prepare", "input_policy", "project_upstream", "forward", "release"}
	if !reflect.DeepEqual(driver.calls, want) {
		t.Fatalf("calls = %v, want %v", driver.calls, want)
	}
}

func TestExchangeWorkflowUseCaseStopsWhenPrepareFails(t *testing.T) {
	driver := &recordingExchangeWorkflowDriver{}

	completed := (ExchangeWorkflowUseCase{}).Run(context.Background(), driver)

	if completed {
		t.Fatal("Run() = true, want false")
	}
	want := []string{"prepare"}
	if !reflect.DeepEqual(driver.calls, want) {
		t.Fatalf("calls = %v, want %v", driver.calls, want)
	}
}

func TestExchangeWorkflowUseCaseReleasesAfterPolicyFailure(t *testing.T) {
	driver := &recordingExchangeWorkflowDriver{prepareOK: true}

	completed := (ExchangeWorkflowUseCase{}).Run(context.Background(), driver)

	if completed {
		t.Fatal("Run() = true, want false")
	}
	want := []string{"prepare", "input_policy", "release"}
	if !reflect.DeepEqual(driver.calls, want) {
		t.Fatalf("calls = %v, want %v", driver.calls, want)
	}
}

func TestExchangeWorkflowUseCaseReleasesAfterProjectionFailure(t *testing.T) {
	driver := &recordingExchangeWorkflowDriver{prepareOK: true, inputOK: true}

	completed := (ExchangeWorkflowUseCase{}).Run(context.Background(), driver)

	if completed {
		t.Fatal("Run() = true, want false")
	}
	want := []string{"prepare", "input_policy", "project_upstream", "release"}
	if !reflect.DeepEqual(driver.calls, want) {
		t.Fatalf("calls = %v, want %v", driver.calls, want)
	}
}

func TestExchangeWorkflowUseCaseRejectsNilDriver(t *testing.T) {
	if (ExchangeWorkflowUseCase{}).Run(context.Background(), nil) {
		t.Fatal("Run(nil) = true, want false")
	}
}

type recordingExchangeWorkflowDriver struct {
	prepareOK bool
	inputOK   bool
	projectOK bool
	calls     []string
}

func (d *recordingExchangeWorkflowDriver) PrepareExchange(context.Context) (func(), bool) {
	d.calls = append(d.calls, "prepare")
	if !d.prepareOK {
		return nil, false
	}
	return func() {
		d.calls = append(d.calls, "release")
	}, true
}

func (d *recordingExchangeWorkflowDriver) ApplyInputPolicy(context.Context) bool {
	d.calls = append(d.calls, "input_policy")
	return d.inputOK
}

func (d *recordingExchangeWorkflowDriver) ProjectUpstreamRequest(context.Context) bool {
	d.calls = append(d.calls, "project_upstream")
	return d.projectOK
}

func (d *recordingExchangeWorkflowDriver) ForwardExchange(context.Context) {
	d.calls = append(d.calls, "forward")
}
