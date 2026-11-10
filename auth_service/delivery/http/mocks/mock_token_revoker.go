// MockTokenRevoker is a gomock-compatible mock implementation of TokenRevoker.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

type MockTokenRevoker struct {
	ctrl     *gomock.Controller
	recorder *MockTokenRevokerMockRecorder
}
type MockTokenRevokerMockRecorder struct{ mock *MockTokenRevoker }

func NewMockTokenRevoker(ctrl *gomock.Controller) *MockTokenRevoker {
	mock := &MockTokenRevoker{ctrl: ctrl}
	mock.recorder = &MockTokenRevokerMockRecorder{mock: mock}
	return mock
}
func (m *MockTokenRevoker) EXPECT() *MockTokenRevokerMockRecorder { return m.recorder }
func (m *MockTokenRevoker) Revoke(ctx context.Context, token string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Revoke", ctx, token)
	ret0, _ := ret[0].(error)
	return ret0
}
func (mr *MockTokenRevokerMockRecorder) Revoke(ctx, token any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Revoke", reflect.TypeOf((*MockTokenRevoker)(nil).Revoke), ctx, token)
}
