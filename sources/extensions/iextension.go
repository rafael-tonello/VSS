package extensions

import (
	"rtonello/vss/sources/misc"
	"rtonello/vss/sources/services/apis"
)

type WhatDo int

const (
	WhatDoContinue              WhatDo = iota //continue with normal processing
	WhatDoReturnFromIntercepted               //abort the normal processing (but only return error if Err is not nil)
)

type ExtenstionsSetVarResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err      error
	NewName  string
	NewValue misc.DynamicVar
}

type ExtenstionsGetVarResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err             error
	NewName         string
	NewDefaultValue misc.DynamicVar

	//if WhatDo is WhatDoReturn and Err is nil, this holds the new value to be returned to the caller
	ReturnValues []misc.Tuple[misc.DynamicVar]
}

type ExtensionsDelVarResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err     error
	NewName string
}

type ExtensionsGetChildsResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err           error
	NewParentName string
	NewValue      misc.DynamicVar

	//if WhatDo is WhatDoReturn and Err is nil, this holds the new value to be returned to the caller
	ReturnValues []string
}

type ExtensionsLockVarResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err          error
	NewName      string
	NewTimeoutMs uint
}

type ExtensionsUnlockVarResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err     error
	NewName string
}

type ExtensionsIsVarLockedResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err     error
	NewName string

	//if WhatDo is WhatDoReturn and Err is nil, this holds the new value to be returned to the caller
	ReturnValue bool
}

type ExtensionsObserveVarResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err                     error
	NewVarName              string
	NewClientId             string
	NewCustomIdsAndMetainfo string
}

type ExtensionsStopObservingVarResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err                     error
	NewVarName              string
	NewClientId             string
	NewCustomIdsAndMetainfo string
}

type ExtensionsApiStartedResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err error
}

type ExtensionsClientConnectedResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err                    error
	NewClientId            string
	NewApiImplementationId int
}

type ExtensionsClientDeletedResult struct {
	WhatDo WhatDo
	//if not nil, the function must return imediately with this error, event if WhatDo is WhatDoContinue
	Err         error
	NewClientId string
}

type IExtension interface {

	//could change parameters and do a custom return
	BeforeSetVar(name string, value misc.DynamicVar, queryHeaders map[string]string) chan ExtenstionsSetVarResult

	//could change the return value
	//this will not be called if BeforeSetVar forces return (by return errors or using WhatDoReturn == WhatDoReturnFromIntercepted)
	AfterSetVar(name string, value misc.DynamicVar, queryHeaders map[string]string, returnValue error) chan ExtenstionsSetVarResult

	//could change parameters and do a custom return
	BeforeGetVars(name string, defaultValue misc.DynamicVar, queryHeaders map[string]string) chan ExtenstionsGetVarResult

	//could change the return value
	//this will not be called if BeforeGetVars forces return (by return errors or using WhatDoReturn == WhatDoReturnFromIntercepted)
	AfterGetVars(name string, defaultValue misc.DynamicVar, queryHeaders map[string]string, returnValues []misc.Tuple[misc.DynamicVar]) chan ExtenstionsGetVarResult

	//could change parameters and do a custom return
	BeforeDelVar(name string, queryHeaders map[string]string) chan ExtensionsDelVarResult

	//could change the return value
	//this will not be called if BeforeDelVar forces return (by return errors or using WhatDoReturn == WhatDoReturnFromIntercepted)
	AfterDelVar(name string, queryHeaders map[string]string, returnValue error) chan ExtensionsDelVarResult

	//could change parameters and do a custom return
	BeforeGetChildsOfVar(parentName string, queryHeaders map[string]string) chan ExtensionsGetChildsResult

	//could change the return value
	//this will not be called if BeforeGetChildsOfVar forces return (by return errors or using WhatDoReturn == WhatDoReturnFromIntercepted)
	AfterGetChildsOfVar(parentName string, queryHeaders map[string]string, returnValue ExtensionsGetChildsResult) chan ExtensionsGetChildsResult

	//could change parameters and do a custom return
	BeforeLockVar(varName string, timeoutMs uint, queryHeaders map[string]string) chan ExtensionsLockVarResult

	//could change the return value
	//this will not be called if BeforeLockVar forces return (by return errors or using WhatDoReturn == WhatDoReturnFromIntercepted)
	AfterLockVar(varName string, timeoutMs uint, queryHeaders map[string]string, returnValue error) chan ExtensionsLockVarResult

	//could change parameters and do a custom return
	BeforeUnlockVar(varName string, queryHeaders map[string]string) chan ExtensionsUnlockVarResult

	//could change the return value
	//this will not be called if BeforeUnlockVar forces return (by return errors or using WhatDoReturn == WhatDoReturnFromIntercepted)
	AfterUnlockVar(varName string, queryHeaders map[string]string, returnValue error) chan ExtensionsUnlockVarResult

	//could change parameters and do a custom return
	BeforeIsVarLocked(varName string, queryHeaders map[string]string) chan ExtensionsIsVarLockedResult

	//could change the return value
	//this will not be called if BeforeIsVarLocked forces return (by return errors or using WhatDoReturn == WhatDoReturnFromIntercepted)
	AfterIsVarLocked(varName string, queryHeaders map[string]string, returnValue bool) chan ExtensionsIsVarLockedResult

	//could change parameters and do a custom return
	BeforeObserveVar(varName, clientId, customIdsAndMetainfo string, api apis.IApi, queryHeaders map[string]string) chan ExtensionsObserveVarResult

	//could change the return value
	//this will not be called if BeforeObserveVar forces return (by return errors or using WhatDoReturn == WhatDoReturnFromIntercepted)
	AfterObserveVar(varName, clientId, customIdsAndMetainfo string, api apis.IApi, queryHeaders map[string]string) chan ExtensionsObserveVarResult

	//could change parameters and do a custom return
	BeforeStopObservingVar(varName, clientId, customIdsAndMetainfo string, api apis.IApi, queryHeaders map[string]string) chan ExtensionsStopObservingVarResult

	//could change the return value
	//this will not be called if BeforeStopObservingVar forces return (by return errors or using WhatDoReturn == WhatDoReturnFromIntercepted)
	AfterStopObservingVar(varName, clientId, customIdsAndMetainfo string, api apis.IApi, queryHeaders map[string]string) chan ExtensionsStopObservingVarResult

	//could change parameters and do a custom return
	BeforeApiStarted(api apis.IApi) chan ExtensionsApiStartedResult

	//could change the return value
	//this will not be called if BeforeApiStarted forces return (by return errors or using WhatDoReturn == WhatDoReturnFromIntercepted)
	AfterApiStarted(api apis.IApi) chan ExtensionsApiStartedResult

	//could change parameters and do a custom return
	BeforeClientConnected(clientId string, api apis.IApi, queryHeaders map[string]string) chan ExtensionsClientConnectedResult

	//could change the return value
	//this will not be called if BeforeClientConnected forces return (by return errors or using WhatDoReturn == WhatDoReturnFromIntercepted)
	AfterClientConnected(clientId string, api apis.IApi, queryHeaders map[string]string, implementationId int) chan ExtensionsClientConnectedResult

	BeforeClientDeleted(clientId string, api apis.IApi, queryHeaders map[string]string) chan ExtensionsClientDeletedResult

	AfterClientDeleted(clientId string, api apis.IApi, queryHeaders map[string]string) chan ExtensionsClientDeletedResult
}
