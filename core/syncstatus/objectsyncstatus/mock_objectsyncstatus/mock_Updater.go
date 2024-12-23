// Code generated by mockery. DO NOT EDIT.

package mock_objectsyncstatus

import (
	app "github.com/anyproto/any-sync/app"
	domain "github.com/anyproto/anytype-heart/core/domain"
	mock "github.com/stretchr/testify/mock"
)

// MockUpdater is an autogenerated mock type for the Updater type
type MockUpdater struct {
	mock.Mock
}

type MockUpdater_Expecter struct {
	mock *mock.Mock
}

func (_m *MockUpdater) EXPECT() *MockUpdater_Expecter {
	return &MockUpdater_Expecter{mock: &_m.Mock}
}

// Init provides a mock function with given fields: a
func (_m *MockUpdater) Init(a *app.App) error {
	ret := _m.Called(a)

	if len(ret) == 0 {
		panic("no return value specified for Init")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*app.App) error); ok {
		r0 = rf(a)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockUpdater_Init_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Init'
type MockUpdater_Init_Call struct {
	*mock.Call
}

// Init is a helper method to define mock.On call
//   - a *app.App
func (_e *MockUpdater_Expecter) Init(a interface{}) *MockUpdater_Init_Call {
	return &MockUpdater_Init_Call{Call: _e.mock.On("Init", a)}
}

func (_c *MockUpdater_Init_Call) Run(run func(a *app.App)) *MockUpdater_Init_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*app.App))
	})
	return _c
}

func (_c *MockUpdater_Init_Call) Return(err error) *MockUpdater_Init_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockUpdater_Init_Call) RunAndReturn(run func(*app.App) error) *MockUpdater_Init_Call {
	_c.Call.Return(run)
	return _c
}

// Name provides a mock function with given fields:
func (_m *MockUpdater) Name() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Name")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockUpdater_Name_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Name'
type MockUpdater_Name_Call struct {
	*mock.Call
}

// Name is a helper method to define mock.On call
func (_e *MockUpdater_Expecter) Name() *MockUpdater_Name_Call {
	return &MockUpdater_Name_Call{Call: _e.mock.On("Name")}
}

func (_c *MockUpdater_Name_Call) Run(run func()) *MockUpdater_Name_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockUpdater_Name_Call) Return(name string) *MockUpdater_Name_Call {
	_c.Call.Return(name)
	return _c
}

func (_c *MockUpdater_Name_Call) RunAndReturn(run func() string) *MockUpdater_Name_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateDetails provides a mock function with given fields: objectId, status, spaceId
func (_m *MockUpdater) UpdateDetails(objectId string, status domain.ObjectSyncStatus, spaceId string) {
	_m.Called(objectId, status, spaceId)
}

// MockUpdater_UpdateDetails_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateDetails'
type MockUpdater_UpdateDetails_Call struct {
	*mock.Call
}

// UpdateDetails is a helper method to define mock.On call
//   - objectId string
//   - status domain.ObjectSyncStatus
//   - spaceId string
func (_e *MockUpdater_Expecter) UpdateDetails(objectId interface{}, status interface{}, spaceId interface{}) *MockUpdater_UpdateDetails_Call {
	return &MockUpdater_UpdateDetails_Call{Call: _e.mock.On("UpdateDetails", objectId, status, spaceId)}
}

func (_c *MockUpdater_UpdateDetails_Call) Run(run func(objectId string, status domain.ObjectSyncStatus, spaceId string)) *MockUpdater_UpdateDetails_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(domain.ObjectSyncStatus), args[2].(string))
	})
	return _c
}

func (_c *MockUpdater_UpdateDetails_Call) Return() *MockUpdater_UpdateDetails_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockUpdater_UpdateDetails_Call) RunAndReturn(run func(string, domain.ObjectSyncStatus, string)) *MockUpdater_UpdateDetails_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockUpdater creates a new instance of MockUpdater. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockUpdater(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockUpdater {
	mock := &MockUpdater{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
