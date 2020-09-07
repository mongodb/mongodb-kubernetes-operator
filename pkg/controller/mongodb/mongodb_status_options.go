package mongodb

import (
	"fmt"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// optionBuilder is in charge of constructing a slice of options that
// will be applied on top of the MongoDB resource that has been provided
type optionBuilder struct {
	mdb          *mdbv1.MongoDB
	retrySeconds int
	options      []status.Option
}

// newOptionBuilder returns an initialized optionBuilder
func newOptionBuilder(mdb *mdbv1.MongoDB) *optionBuilder {
	return &optionBuilder{
		mdb:     mdb,
		options: []status.Option{},
	}
}

// toStatusOptions should be called on terminal operations
// these operations will return the final set of options that will
// be applied the status of the resource
func (o *optionBuilder) toStatusOptions() status.Option {
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

func (o *optionBuilder) success() status.Option {
	return o.withMembers(o.mdb.Spec.Members).
		withMongoURI(o.mdb.MongoURI()).
		runningPhase()
}

func (o *optionBuilder) failed(msg string) status.Option {
	return o.withPhase(mdbv1.Failed, msg).toStatusOptions()
}

func (o *optionBuilder) failedf(msg string, params ...interface{}) status.Option {
	return o.failed(fmt.Sprintf(msg, params...))
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

func (o *optionBuilder) retryAfter(seconds int) *optionBuilder {
	o.retrySeconds = seconds
	o.options = append(o.options,
		retryOption{
			retryAfter: seconds,
		})
	return o
}

func (o *optionBuilder) retryImmediately() *optionBuilder {
	return o.retryAfter(0)
}

func (o *optionBuilder) withMongoURI(uri string) *optionBuilder {
	o.options = append(o.options,
		mongoUriOption{
			mdb:      o.mdb,
			mongoUri: uri,
		})
	return o
}

type mongoUriOption struct {
	mongoUri string
	mdb      *mdbv1.MongoDB
}

func (m mongoUriOption) ApplyOption() {
	m.mdb.Status.MongoURI = m.mongoUri
}

func (m mongoUriOption) GetResult() (reconcile.Result, error) {
	return ok()
}

func (o *optionBuilder) withMembers(members int) *optionBuilder {
	o.options = append(o.options,
		membersOption{
			mdb:     o.mdb,
			members: members,
		})
	return o
}

type membersOption struct {
	members int
	mdb     *mdbv1.MongoDB
}

func (m membersOption) ApplyOption() {
	m.mdb.Status.Members = m.members
}

func (m membersOption) GetResult() (reconcile.Result, error) {
	return ok()
}

func (o *optionBuilder) runningPhase() status.Option {
	return o.withPhase(mdbv1.Running, "").toStatusOptions()
}

func (o *optionBuilder) withPhase(phase mdbv1.Phase, msg string) *optionBuilder {
	o.options = append(o.options,
		phaseOption{
			mdb:          o.mdb,
			phase:        phase,
			message:      msg,
			retrySeconds: o.retrySeconds,
		})
	return o
}

func (o *optionBuilder) pendingPhase(msg string) status.Option {
	return o.withPhase(mdbv1.Pending, msg).toStatusOptions()
}

func (o *optionBuilder) pendingPhasef(msg string, params ...interface{}) status.Option {
	return o.withPhase(mdbv1.Pending, fmt.Sprintf(msg, params...)).toStatusOptions()
}

type phaseOption struct {
	phase        mdbv1.Phase
	message      string
	mdb          *mdbv1.MongoDB
	retrySeconds int
}

func (p phaseOption) ApplyOption() {
	p.mdb.Status.Phase = p.phase
}

func (p phaseOption) GetResult() (reconcile.Result, error) {
	if p.phase == mdbv1.Running {
		return ok()
	}
	if p.phase == mdbv1.Pending {
		if p.message != "" {
			zap.S().Infof(p.message)
		}
		return retry(p.retrySeconds)
	}
	if p.phase == mdbv1.Failed {
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
	return reconcile.Result{Requeue: true, RequeueAfter: time.Second * time.Duration(after)}, nil
}

func failed(msg string, params ...interface{}) (reconcile.Result, error) {
	return reconcile.Result{}, errors.Errorf(msg, params...)
}
