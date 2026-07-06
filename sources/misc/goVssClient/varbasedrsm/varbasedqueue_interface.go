package varbasedsrm

//rsm = remote shared memory
import "rtonello/vss/sources/misc"

// var based remote shared memory
type VarBasedQueue interface {
	GetState(name string) any

	SetState(name string, value any) error

	PostMessage(data misc.DynamicVar) error

	BeginConsume()

	Subscribe(callback func(queue VarBasedQueue, data misc.DynamicVar)) (int, error)

	Unsubscribe(subscriptionId int) error

	UnsubscribeAll() error

	Close() error
}
