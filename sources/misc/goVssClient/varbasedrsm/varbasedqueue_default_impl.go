package varbasedsrm

/* this is the default implementation of VarBasedQueue interface. Unless your srm needs a special implementation to work with
queues, you can just use this one.
*/

import (
	"errors"
	"fmt"
	"os"
	"rtonello/vss/sources/misc"
	"strconv"
	"sync/atomic"
)

type Queue struct {
	cli                  VarBasedRSM
	queueName            string
	IncomingMessages     *misc.Stream[misc.DynamicVar]
	currentlyConsuming   atomic.Bool
	state                map[string]any
	consummingObserverId int
}

// Create a new queue
// after you handle de data events and is read to star consume, call 'Init' method
func NewQueue(srm VarBasedRSM, queueName string) (*Queue, error) {
	ret := Queue{}
	ret.IncomingMessages = misc.NewStream[misc.DynamicVar](false)
	ret.currentlyConsuming = atomic.Bool{}
	ret.currentlyConsuming.Store(false)

	ret.cli = srm
	ret.queueName = queueName
	ret.state = map[string]any{}
	ret.consummingObserverId = -1

	return &ret, nil
}

func (queue *Queue) Close() error {
	if queue.consummingObserverId != -1 {
		queue.cli.Unsubscribe(queue.consummingObserverId)
	}

	return nil
}

func (queue *Queue) PostMessage(data misc.DynamicVar) error {
	err := queue.cli.Lock(queue.queueName+".lock", 5000)
	if err != nil {
		queue.cli.Unlock(queue.queueName + ".lock")
		os.Stderr.WriteString(fmt.Errorf("error locking queue to post message. System will try to continue: %w", err).Error() + "\n")

	}

	queueSize, err := queue.cli.GetVar(queue.queueName + ".count")

	if len(queueSize) == 0 || queueSize[0].Value.GetString() == "" {
		queueSize = []KeyValue{{
			Key:   queue.queueName + ".count",
			Value: misc.NewDynamicVar(misc.WithInt(0)),
		}}
	}

	if err != nil {
		queue.cli.Unlock(queue.queueName + ".lock")
		return fmt.Errorf("error get queue size: %w", err)
	}
	if len(queueSize) <= 0 {
		queue.cli.Unlock(queue.queueName + ".lock")
		return errors.New("error get queue size. Canno get '.count' property of the queue")
	}

	queue.cli.SetVar(queue.queueName+".items."+queueSize[0].Value.GetString(), data)
	queue.cli.SetVar(queue.queueName+".count", misc.NewDynamicVar(misc.WithInt(queueSize[0].Value.GetInt64()+1)))
	queue.cli.Unlock(queue.queueName + ".lock")

	return nil
}

func (queue *Queue) BeginConsume() {
	queue.consummingObserverId, _ = queue.cli.Subscribe(queue.queueName+".count", func(name string, value misc.DynamicVar) {
		go func() { queue.consumeMessages() }()
	})
}

func (queue *Queue) consumeMessages() {
	if queue.currentlyConsuming.Load() {
		return
	}

	queue.currentlyConsuming.Store(true)

	for {
		err := queue.cli.Lock(queue.queueName+".lock", 1000)

		if err != nil {
			os.Stderr.WriteString("error locking var: " + err.Error() + ". System will try to continue" + "\n")
		}

		next, err, endOfQueue := queue.tryGetNextMessage()

		queue.cli.Unlock(queue.queueName + ".lock")

		if err != nil {
			//fmt.Println("returned error: " + err.Error())
			continue
		}

		if endOfQueue {
			break
		}

		fmt.Println("message received from app: " + next.GetString())
		queue.processReceivedData(next)
	}
	queue.currentlyConsuming.Store(false)
}

func (queue *Queue) tryGetNextMessage() (misc.DynamicVar, error, bool) {
	posV, err := queue.cli.GetVar(queue.queueName + ".next")
	if err != nil {
		fmt.Println("error getting next position")
		return misc.NewEmptyDynamicVar(), fmt.Errorf("error getting next position: %w", err), false
	}

	if len(posV) == 0 {
		return misc.NewEmptyDynamicVar(), fmt.Errorf("error getting next position: no vars returned"), false
	}

	if posV[0].Value.GetString() == "" {
		posV[0].Value = misc.NewDynamicVar(misc.WithInt(0))
	}

	countV, err := queue.cli.GetVar(queue.queueName + ".count")
	if err != nil {
		return misc.NewEmptyDynamicVar(), fmt.Errorf("error getting message count: %w", err), false
	}
	if len(countV) == 0 || countV[0].Value.GetString() == "" {
		return misc.NewEmptyDynamicVar(), fmt.Errorf("error getting message count: no vars returned"), false
	}

	pos := posV[0].Value.GetInt64()
	count := countV[0].Value.GetInt64()

	if pos >= count {
		return misc.NewEmptyDynamicVar(), nil, true
	}

	//fmt.Println("getting " + queue.queueName + ".items." + strconv.FormatInt(pos, 10))
	data, err := queue.cli.GetVar(queue.queueName + ".items." + strconv.FormatInt(pos, 10))
	if err != nil || len(data) == 0 {
		if err == nil {
			err = errors.New("no data")
		}

		return misc.NewEmptyDynamicVar(), fmt.Errorf("error geting var: %w", err), false
	}

	newNextVal := misc.NewDynamicVar(misc.WithInt(pos + 1))
	queue.cli.SetVar((queue.queueName + ".next"), newNextVal)
	_ = queue.cli.DeleteVar(queue.queueName + ".items." + strconv.FormatInt(pos, 10) + ".*")

	return data[0].Value, nil, false
}

func (queue *Queue) processReceivedData(data misc.DynamicVar) {
	queue.IncomingMessages.Stream(data)
}

func (queue *Queue) GetIncomingMessagesStream() (*misc.Stream[misc.DynamicVar], error) {
	return queue.IncomingMessages, nil
}

func (queue *Queue) Subscribe(callback func(queue VarBasedQueue, data misc.DynamicVar)) (int, error) {
	id := queue.IncomingMessages.Subscribe(func(data misc.DynamicVar) {
		callback(queue, data)
	})
	return id, nil
}

func (queue *Queue) Unsubscribe(subscriptionId int) error {
	queue.IncomingMessages.Unsubscribe(subscriptionId)
	return nil
}

func (queue *Queue) UnsubscribeAll() error {
	queue.IncomingMessages.UnsubscribeAll()
	return nil
}

func (queue *Queue) GetState(name string) any {
	return queue.state[name]
}

func (queue *Queue) SetState(name string, value any) error {
	queue.state[name] = value
	return nil
}

var _ VarBasedQueue = (*Queue)(nil)
