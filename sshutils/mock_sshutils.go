package sshutils

// import (
//     "github.com/golang/mock/gomock"
//     "golang.org/x/crypto/ssh"
// )

// // MockSSHClient is a mock of SSHClient interface
// type MockSSHClient struct {
//     ctrl     *gomock.Controller
//     recorder *MockSSHClientMockRecorder
// }

// // MockSSHClientMockRecorder is the mock recorder for MockSSHClient
// type MockSSHClientMockRecorder struct {
//     mock *MockSSHClient
// }

// // NewMockSSHClient creates a new mock instance
// func NewMockSSHClient(ctrl *gomock.Controller) *MockSSHClient {
//     mock := &MockSSHClient{ctrl: ctrl}
//     mock.recorder = &MockSSHClientMockRecorder{mock}
//     return mock
// }

// // EXPECT returns an object that allows the caller to indicate expected use
// func (m *MockSSHClient) EXPECT() *MockSSHClientMockRecorder {
//     return m.recorder
// }

// // NewSession mocks base method
// func (m *MockSSHClient) NewSession() (*ssh.Session, error) {
//     m.ctrl.T.Helper()
//     ret := m.ctrl.Call(m, "NewSession")
//     ret0, _ := ret[0].(*ssh.Session)
//     ret1, _ := ret[1].(error)
//     return ret0, ret1
// }

// // NewSession indicates an expected call of NewSession
// func (mr *MockSSHClientMockRecorder) NewSession() *gomock.Call {
//     mr.mock.ctrl.T.Helper()
//     return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewSession", reflect.TypeOf((*MockSSHClient)(nil).NewSession))
// }

// // Close mocks base method
// func (m *MockSSHClient) Close() error {
//     m.ctrl.T.Helper()
//     ret := m.ctrl.Call(m, "Close")
//     ret0, _ := ret[0].(error)
//     return ret0
// }

// // Close indicates an expected call of Close
// func (mr *MockSSHClientMockRecorder) Close() *gomock.Call {
//     mr.mock.ctrl.T.Helper()
//     return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockSSHClient)(nil).Close))
// }

// // MockSSHSession is a mock of ssh.Session
// type MockSSHSession struct {
//     ctrl     *gomock.Controller
//     recorder *MockSSHSessionMockRecorder
// }

// // MockSSHSessionMockRecorder is the mock recorder for MockSSHSession
// type MockSSHSessionMockRecorder struct {
//     mock *MockSSHSession
// }

// // NewMockSSHSession creates a new mock instance
// func NewMockSSHSession(ctrl *gomock.Controller) *MockSSHSession {
//     mock := &MockSSHSession{ctrl: ctrl}
//     mock.recorder = &MockSSHSessionMockRecorder{mock}
//     return mock
// }

// // EXPECT returns an object that allows the caller to indicate expected use
// func (m *MockSSHSession) EXPECT() *MockSSHSessionMockRecorder {
//     return m.recorder
// }

// // Run mocks base method
// func (m *MockSSHSession) Run(cmd string) error {
//     m.ctrl.T.Helper()
//     ret := m.ctrl.Call(m, "Run", cmd)
//     ret0, _ := ret[0].(error)
//     return ret0
// }

// // Run indicates an expected call of Run
// func (mr *MockSSHSessionMockRecorder) Run(cmd interface{}) *gomock.Call {
//     mr.mock.ctrl.T.Helper()
//     return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockSSHSession)(nil).Run), cmd)
// }

// // Close mocks base method
// func (m *MockSSHSession) Close() error {
//     m.ctrl.T.Helper()
//     ret := m.ctrl.Call(m, "Close")
//     ret0, _ := ret[0].(error)
//     return ret0
// }

// // Close indicates an expected call of Close
// func (mr *MockSSHSessionMockRecorder) Close() *gomock.Call {
//     mr.mock.ctrl.T.Helper()
//     return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockSSHSession)(nil).Close))
// }

// // CombinedOutput mocks base method
// func (m *MockSSHSession) CombinedOutput(cmd string) ([]byte, error) {
//     m.ctrl.T.Helper()
//     ret := m.ctrl.Call(m, "CombinedOutput", cmd)
//     ret0, _ := ret[0].([]byte)
//     ret1, _ := ret[1].(error)
//     return ret0, ret1
// }

// // CombinedOutput indicates an expected call of CombinedOutput
// func (mr *MockSSHSessionMockRecorder) CombinedOutput(cmd interface{}) *gomock.Call {
//     mr.mock.ctrl.T.Helper()
//     return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CombinedOutput", reflect.TypeOf((*MockSSHSession)(nil).CombinedOutput), cmd)
// }
