package mongodb

import (
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"go.uber.org/zap"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// severity indicates the severity level
// at which the message should be logged
type severity string

const (
	Info  severity = "INFO"
	Warn  severity = "WARN"
	Error severity = "ERROR"
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

type retryOption struct {
	retryAfter int
}

func (r retryOption) ApplyOption(_ *mdbv1.MongoDB) {
	// has no impact on the resource status itself
}

func (r retryOption) GetResult() (reconcile.Result, error) {
	return retryResult(r.retryAfter)
}

func (o *optionBuilder) retryAfter(seconds int) *optionBuilder {
	o.options = append(o.options,
		retryOption{
			retryAfter: seconds,
		})
	return o
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

func (o *optionBuilder) withMembers(members int) *optionBuilder {
	o.options = append(o.options,
		membersOption{
			members: members,
		})
	return o
}

type membersOption struct {
	members int
}

func (m membersOption) ApplyOption(mdb *mdbv1.MongoDB) {
	mdb.Status.Members = m.members
}

func (m membersOption) GetResult() (reconcile.Result, error) {
	return okResult()
}

func (o *optionBuilder) withPhase(phase mdbv1.Phase) *optionBuilder {
	o.options = append(o.options,
		phaseOption{
			phase: phase,
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
		zap.S().Error(m.message)
	}
	if m.message.severityLevel == Warn {
		zap.S().Warn(m.message)
	}
	if m.message.severityLevel == Info {
		zap.S().Info(m.message)
	}
}

func (m messageOption) GetResult() (reconcile.Result, error) {
	return okResult()
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

type phaseOption struct {
	phase mdbv1.Phase
}

func (p phaseOption) ApplyOption(mdb *mdbv1.MongoDB) {
	mdb.Status.Phase = p.phase
}

func (p phaseOption) GetResult() (reconcile.Result, error) {
	if p.phase == mdbv1.Running {
		return okResult()
	}
	if p.phase == mdbv1.Pending {
		return retryResult(10)
	}
	if p.phase == mdbv1.Failed {
		return failedResult()
	}
	return okResult()
}

// helper functions which return reconciliation results which should be
// returned from the main reconciliation loop

func okResult() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func retryResult(after int) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: time.Second * time.Duration(after)}, nil
}

func failedResult() (reconcile.Result, error) {
	// the error returned from this function will cause the reconciler to requeue
	// the reconciliation, but the message itself isn't what ends up on the status of the resource
	// that must be set with withMessage(severityLevel, msg)
	return reconcile.Result{}, errors.New("error during reconciliation")
}
