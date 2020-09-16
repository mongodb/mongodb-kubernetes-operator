package mongodb

import (
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"go.uber.org/zap"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// severity indicates the severity level
// at which the message should be logged
type severity string

const (
	Info  severity = "INFO"
	Debug severity = "DEBUG"
	Warn  severity = "WARN"
	Error severity = "ERROR"
	None  severity = "NONE"
)

// optionBuilder is in charge of constructing a slice of options that
// will be applied on top of the MongoDB resource that has been provided
type optionBuilder struct {
	options []status.Option
}

// GetOptions implements the OptionBuilder interface
func (o *optionBuilder) GetOptions() []status.Option {
	return o.options
}

// options returns an initialized optionBuilder
func statusOptions() *optionBuilder {
	return &optionBuilder{
		options: []status.Option{},
	}
}

func (o *optionBuilder) withMongoURI(uri string) *optionBuilder {
	o.options = append(o.options,
		mongoUriOption{
			mongoUri: uri,
		})
	return o
}

type mongoUriOption struct {
	mongoUri string
}

func (m mongoUriOption) ApplyOption(mdb *mdbv1.MongoDB) {
	mdb.Status.MongoURI = m.mongoUri
}

func (m mongoUriOption) GetResult() (reconcile.Result, error) {
	return okResult()
}

func (o *optionBuilder) withPhase(phase mdbv1.Phase, retryAfter int) *optionBuilder {
	o.options = append(o.options,
		phaseOption{
			phase:      phase,
			retryAfter: retryAfter,
		})
	return o
}

type message struct {
	messageString string
	severityLevel severity
}

type messageOption struct {
	message message
}

func (m messageOption) ApplyOption(mdb *mdbv1.MongoDB) {
	mdb.Status.Message = m.message.messageString
	if m.message.severityLevel == Error {
		zap.S().Error(m.message.messageString)
	}
	if m.message.severityLevel == Warn {
		zap.S().Warn(m.message.messageString)
	}
	if m.message.severityLevel == Info {
		zap.S().Info(m.message.messageString)
	}
	if m.message.severityLevel == Debug {
		zap.S().Debug(m.message.messageString)
	}
}

func (m messageOption) GetResult() (reconcile.Result, error) {
	return okResult()
}

func (o *optionBuilder) withAutomationConfigMembers(members int) *optionBuilder {
	o.options = append(o.options, automationConfigReplicasOption{
		replicaSetMembers: members,
	})
	return o
}

func (o *optionBuilder) withStatefulSetReplicas(members int) *optionBuilder {
	o.options = append(o.options, statefulSetReplicasOption{
		replicas: members,
	})
	return o
}

func (o *optionBuilder) withMessage(severityLevel severity, msg string) *optionBuilder {
	o.options = append(o.options, messageOption{
		message: message{
			messageString: msg,
			severityLevel: severityLevel,
		},
	})
	return o
}

func (o *optionBuilder) withFailedPhase() *optionBuilder {
	return o.withPhase(mdbv1.Failed, 0)
}

func (o *optionBuilder) withPendingPhase(retryAfter int) *optionBuilder {
	return o.withPhase(mdbv1.Pending, retryAfter)
}

func (o *optionBuilder) withRunningPhase() *optionBuilder {
	return o.withPhase(mdbv1.Running, -1)
}

type phaseOption struct {
	phase      mdbv1.Phase
	retryAfter int
}

func (p phaseOption) ApplyOption(mdb *mdbv1.MongoDB) {
	mdb.Status.Phase = p.phase
}

func (p phaseOption) GetResult() (reconcile.Result, error) {
	if p.phase == mdbv1.Running {
		return okResult()
	}
	if p.phase == mdbv1.Pending {
		return retryResult(p.retryAfter)
	}
	if p.phase == mdbv1.Failed {
		return failedResult()
	}
	return okResult()
}

type automationConfigReplicasOption struct {
	replicaSetMembers int
}

func (a automationConfigReplicasOption) ApplyOption(mdb *mdbv1.MongoDB) {
	mdb.Status.CurrentReplicaSetMembers = a.replicaSetMembers
}

func (a automationConfigReplicasOption) GetResult() (reconcile.Result, error) {
	return okResult()
}

type statefulSetReplicasOption struct {
	replicas int
}

func (s statefulSetReplicasOption) ApplyOption(mdb *mdbv1.MongoDB) {
	mdb.Status.CurrentStatefulSetReplicas = s.replicas
}

func (s statefulSetReplicasOption) GetResult() (reconcile.Result, error) {
	return okResult()
}

// helper functions which return reconciliation results which should be
// returned from the main reconciliation loop

func okResult() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func retryResult(after int) (reconcile.Result, error) {
	return reconcile.Result{Requeue: true, RequeueAfter: time.Second * time.Duration(after)}, nil
}

func failedResult() (reconcile.Result, error) {
	return retryResult(0)
}
