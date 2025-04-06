// Code generated by MockGen. DO NOT EDIT.
// Source: pkg/eventemitter/eventemitter.go
//
// Generated by this command:
//
//	mockgen -destination=pkg/mock/mock_eventemitter/mock_interface.go -package=mock_eventemitter -source=pkg/eventemitter/eventemitter.go
//

// Package mock_eventemitter is a generated GoMock package.
package mock_eventemitter

import (
	context "context"
	reflect "reflect"

	v1alpha1 "github.com/kedacore/keda/v2/apis/eventing/v1alpha1"
	eventdata "github.com/kedacore/keda/v2/pkg/eventemitter/eventdata"
	gomock "go.uber.org/mock/gomock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// MockEventHandler is a mock of EventHandler interface.
type MockEventHandler struct {
	ctrl     *gomock.Controller
	recorder *MockEventHandlerMockRecorder
	isgomock struct{}
}

// MockEventHandlerMockRecorder is the mock recorder for MockEventHandler.
type MockEventHandlerMockRecorder struct {
	mock *MockEventHandler
}

// NewMockEventHandler creates a new mock instance.
func NewMockEventHandler(ctrl *gomock.Controller) *MockEventHandler {
	mock := &MockEventHandler{ctrl: ctrl}
	mock.recorder = &MockEventHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEventHandler) EXPECT() *MockEventHandlerMockRecorder {
	return m.recorder
}

// DeleteCloudEventSource mocks base method.
func (m *MockEventHandler) DeleteCloudEventSource(cloudEventSource v1alpha1.CloudEventSourceInterface) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteCloudEventSource", cloudEventSource)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteCloudEventSource indicates an expected call of DeleteCloudEventSource.
func (mr *MockEventHandlerMockRecorder) DeleteCloudEventSource(cloudEventSource any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteCloudEventSource", reflect.TypeOf((*MockEventHandler)(nil).DeleteCloudEventSource), cloudEventSource)
}

// Emit mocks base method.
func (m *MockEventHandler) Emit(object runtime.Object, namespace, eventType string, cloudeventType v1alpha1.CloudEventType, reason, message string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Emit", object, namespace, eventType, cloudeventType, reason, message)
}

// Emit indicates an expected call of Emit.
func (mr *MockEventHandlerMockRecorder) Emit(object, namespace, eventType, cloudeventType, reason, message any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Emit", reflect.TypeOf((*MockEventHandler)(nil).Emit), object, namespace, eventType, cloudeventType, reason, message)
}

// HandleCloudEventSource mocks base method.
func (m *MockEventHandler) HandleCloudEventSource(ctx context.Context, cloudEventSource v1alpha1.CloudEventSourceInterface) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HandleCloudEventSource", ctx, cloudEventSource)
	ret0, _ := ret[0].(error)
	return ret0
}

// HandleCloudEventSource indicates an expected call of HandleCloudEventSource.
func (mr *MockEventHandlerMockRecorder) HandleCloudEventSource(ctx, cloudEventSource any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HandleCloudEventSource", reflect.TypeOf((*MockEventHandler)(nil).HandleCloudEventSource), ctx, cloudEventSource)
}

// MockEventDataHandler is a mock of EventDataHandler interface.
type MockEventDataHandler struct {
	ctrl     *gomock.Controller
	recorder *MockEventDataHandlerMockRecorder
	isgomock struct{}
}

// MockEventDataHandlerMockRecorder is the mock recorder for MockEventDataHandler.
type MockEventDataHandlerMockRecorder struct {
	mock *MockEventDataHandler
}

// NewMockEventDataHandler creates a new mock instance.
func NewMockEventDataHandler(ctrl *gomock.Controller) *MockEventDataHandler {
	mock := &MockEventDataHandler{ctrl: ctrl}
	mock.recorder = &MockEventDataHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEventDataHandler) EXPECT() *MockEventDataHandlerMockRecorder {
	return m.recorder
}

// CloseHandler mocks base method.
func (m *MockEventDataHandler) CloseHandler() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "CloseHandler")
}

// CloseHandler indicates an expected call of CloseHandler.
func (mr *MockEventDataHandlerMockRecorder) CloseHandler() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseHandler", reflect.TypeOf((*MockEventDataHandler)(nil).CloseHandler))
}

// EmitEvent mocks base method.
func (m *MockEventDataHandler) EmitEvent(eventData eventdata.EventData, failureFunc func(eventdata.EventData, error)) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "EmitEvent", eventData, failureFunc)
}

// EmitEvent indicates an expected call of EmitEvent.
func (mr *MockEventDataHandlerMockRecorder) EmitEvent(eventData, failureFunc any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EmitEvent", reflect.TypeOf((*MockEventDataHandler)(nil).EmitEvent), eventData, failureFunc)
}

// GetActiveStatus mocks base method.
func (m *MockEventDataHandler) GetActiveStatus() v1.ConditionStatus {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetActiveStatus")
	ret0, _ := ret[0].(v1.ConditionStatus)
	return ret0
}

// GetActiveStatus indicates an expected call of GetActiveStatus.
func (mr *MockEventDataHandlerMockRecorder) GetActiveStatus() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetActiveStatus", reflect.TypeOf((*MockEventDataHandler)(nil).GetActiveStatus))
}

// SetActiveStatus mocks base method.
func (m *MockEventDataHandler) SetActiveStatus(status v1.ConditionStatus) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetActiveStatus", status)
}

// SetActiveStatus indicates an expected call of SetActiveStatus.
func (mr *MockEventDataHandlerMockRecorder) SetActiveStatus(status any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetActiveStatus", reflect.TypeOf((*MockEventDataHandler)(nil).SetActiveStatus), status)
}
