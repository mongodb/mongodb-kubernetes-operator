package v1

import (
	"fmt"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// OptionBuilder is in charge of constructing a slice of options that
// will be applied on top of the MongoDB resource that has been provided
type OptionBuilder struct {
	mdb     *MongoDB
	options []status.Option
}

// NewOptionBuilder returns an initialized OptionBuilder
func NewOptionBuilder(mdb *MongoDB) *OptionBuilder {
	return &OptionBuilder{
		mdb:     mdb,
		options: []status.Option{},
	}
}

// toStatusOptions should be called on terminal operations
// these operations will return the final set of options that will
// be applied the status of the resource
func (o *OptionBuilder) toStatusOptions() status.Option {
	return multiOption{
		options: o.options,
	}
}

type multiOption struct {
	options []status.Option
}

func (m multiOption) ApplyOption() {
	for _, opt := range m.options {
		opt.ApplyOption()
	}
}

func (m multiOption) GetResult() (reconcile.Result, error) {
	return status.DetermineReconciliationResult(m.options)
}

func (o *OptionBuilder) Success() status.Option {
	return o.WithMembers(o.mdb.Spec.Members).
		WithMongoURI(o.mdb.MongoURI()).
		RunningPhase()
}

func (o *OptionBuilder) Failed(msg string) status.Option {
	return o.withPhase(Failed, msg).toStatusOptions()
}

func (o *OptionBuilder) Failedf(msg string, params ...interface{}) status.Option {
	return o.Failed(fmt.Sprintf(msg, params...))
}

type retryOption struct {
	retryAfter int
}

func (r retryOption) ApplyOption() {
	// has no impact on the resource status itself
}

func (r retryOption) GetResult() (reconcile.Result, error) {
	return retry(r.retryAfter)
}

func (o *OptionBuilder) RetryAfter(seconds int) *OptionBuilder {
	o.options = append(o.options,
		retryOption{
			retryAfter: seconds,
		})
	return o
}

func (o *OptionBuilder) WithMongoURI(uri string) *OptionBuilder {
	o.options = append(o.options,
		mongoUriOption{
			mdb:      o.mdb,
			mongoUri: uri,
		})
	return o
}

type mongoUriOption struct {
	mongoUri string
	mdb      *MongoDB
}

func (m mongoUriOption) ApplyOption() {
	m.mdb.Status.MongoURI = m.mongoUri
}

func (m mongoUriOption) GetResult() (reconcile.Result, error) {
	return ok()
}

func (o *OptionBuilder) WithMembers(members int) *OptionBuilder {
	o.options = append(o.options,
		membersOption{
			mdb:     o.mdb,
			members: members,
		})
	return o
}

type membersOption struct {
	members int
	mdb     *MongoDB
}

func (m membersOption) ApplyOption() {
	m.mdb.Status.Members = m.members
}

func (m membersOption) GetResult() (reconcile.Result, error) {
	return ok()
}

func (o *OptionBuilder) RunningPhase() status.Option {
	return o.withPhase(Running, "").toStatusOptions()
}

func (o *OptionBuilder) withPhase(phase Phase, msg string) *OptionBuilder {
	o.options = append(o.options,
		phaseOption{
			mdb:     o.mdb,
			phase:   phase,
			message: msg,
		})
	return o
}

func (o *OptionBuilder) PendingPhase(msg string) status.Option {
	return o.withPhase(Pending, msg).toStatusOptions()
}

func (o *OptionBuilder) PendingPhasef(msg string, params ...interface{}) status.Option {
	return o.withPhase(Pending, fmt.Sprintf(msg, params...)).toStatusOptions()
}

type phaseOption struct {
	phase   Phase
	message string
	mdb     *MongoDB
}

func (p phaseOption) ApplyOption() {
	p.mdb.Status.Phase = p.phase
}

func (p phaseOption) GetResult() (reconcile.Result, error) {
	if p.phase == Running {
		return ok()
	}
	if p.phase == Pending {
		return retry(10)
	}
	if p.phase == Failed {
		// TODO: don't access global logger here
		zap.S().Errorf(p.message)
		return failed(p.message)
	}
	return ok()
}

func ok() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func retry(after int) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: time.Second * time.Duration(after)}, nil
}

func failed(msg string, params ...interface{}) (reconcile.Result, error) {
	return reconcile.Result{}, errors.Errorf(msg, params...)
}
