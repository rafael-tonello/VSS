package varbasedsrm

//rsm = remote shared memory
import (
	"errors"
	"rtonello/vss/sources/misc"
)

type ConnectionState string

const (
	Connecting   ConnectionState = "Connecting"
	Connected    ConnectionState = "Connected"
	Disconnected ConnectionState = "Disconnected"
)

type KeyValue struct {
	Key   string
	Value misc.DynamicVar
}

type ConnectionStateInfo struct {
	State       ConnectionState
	Description string
}

// var based remote shared memory
type VarBasedRSM interface {
	GetConnectionStateChangedEvent() *misc.Stream[ConnectionStateInfo]
	Connect(serverAndPort string) error

	Var(name string) Var
	GetVar(name string) ([]KeyValue, error)
	GetOneVar(name string) (KeyValue, error)
	SetVar(name string, value misc.DynamicVar) error
	DeleteVar(name string) error

	Lock(lockName string, timeoutMs int) error
	Unlock(lockName string) error
	RunLocked(lockName string, lockTimeoutMs int, failOnLockTimeout bool, callback func() error) error

	Subscribe(varName string, callback func(varName string, newValue misc.DynamicVar)) (int, error)
	Unsubscribe(subscriptionID int) error

	GetChildNames(parentName string) ([]string, error)

	GetQueue(queueName string) (VarBasedQueue, error)
}

type Var struct {
	Name         string
	controller   VarBasedRSM
	subscribeIds []int
}

func NewVar(name string, rsm VarBasedRSM) Var {
	return Var{
		Name:       name,
		controller: rsm,
	}
}

func (instance *Var) GetValue() (misc.DynamicVar, error) {
	result, err := instance.controller.GetVar(instance.Name)
	if err != nil {
		return misc.NewEmptyDynamicVar(), err
	}
	if len(result) == 0 {
		return misc.NewEmptyDynamicVar(), errors.New("no vars returned by the source")
	}
	return result[0].Value, nil
}

func (instance *Var) SetValue(value misc.DynamicVar) error {
	return instance.controller.SetVar(instance.Name, value)
}

func (instance *Var) Delete() error {
	return instance.controller.DeleteVar(instance.Name)
}

func (instance *Var) Lock(timeoutMs int) error {
	return instance.controller.Lock(instance.Name, timeoutMs)
}

func (instance *Var) Unlock() error {
	return instance.controller.Unlock(instance.Name)
}

func (instance *Var) Subscribe(callback func(misc.DynamicVar)) (int, error) {
	subId, err := instance.controller.Subscribe(instance.Name, func(varName string, newValue misc.DynamicVar) {
		callback(newValue)
	})

	if err != nil {
		return -1, err
	}
	instance.subscribeIds = append(instance.subscribeIds, subId)
	return subId, nil
}

func (instance *Var) Unsubscribe(subscriptionID int) error {
	return instance.controller.Unsubscribe(subscriptionID)
}

func (instance *Var) UnsubscribeAll() {
	for _, id := range instance.subscribeIds {
		instance.controller.Unsubscribe(id)
	}
}

func (instance *Var) GetChildNames() ([]string, error) {
	return instance.controller.GetChildNames(instance.Name)
}
