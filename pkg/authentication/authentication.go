package authentication

import (
	"context"
	"fmt"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"

	"k8s.io/apimachinery/pkg/types"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scram"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/x509"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
)

func Enable(ctx context.Context, auth *automationconfig.Auth, secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb authtypes.Configurable, agentCertSecret types.NamespacedName) error {
	scramEnabled := false
	for _, authMode := range mdb.GetAuthOptions().AuthMechanisms {
		switch authMode {
		case constants.Sha1, constants.Sha256:
			if !scramEnabled {
				if err := scram.Enable(ctx, auth, secretGetUpdateCreateDeleter, mdb); err != nil {
					return fmt.Errorf("could not configure scram authentication: %s", err)
				}
				scramEnabled = true
			}
		case constants.X509:
			if err := x509.Enable(ctx, auth, secretGetUpdateCreateDeleter, mdb, agentCertSecret); err != nil {
				return fmt.Errorf("could not configure x509 authentication: %s", err)
			}
		}
	}
	return nil
}

func AddRemovedUsers(auth *automationconfig.Auth, mdb mdbv1.MongoDBCommunity, lastAppliedSpec *mdbv1.MongoDBCommunitySpec) {
	deletedUsers := getRemovedUsersFromSpec(mdb.Spec, lastAppliedSpec)

	auth.UsersDeleted = append(auth.UsersDeleted, deletedUsers...)
}

func getRemovedUsersFromSpec(currentMDB mdbv1.MongoDBCommunitySpec, lastAppliedMDBSpec *mdbv1.MongoDBCommunitySpec) []automationconfig.DeletedUser {
	type user struct {
		db   string
		name string
	}
	m := map[user]bool{}
	var deletedUsers []automationconfig.DeletedUser

	for _, mongoDBUser := range currentMDB.Users {
		if mongoDBUser.DB == constants.ExternalDB {
			continue
		}
		m[user{db: mongoDBUser.DB, name: mongoDBUser.Name}] = true
	}

	for _, mongoDBUser := range lastAppliedMDBSpec.Users {
		if mongoDBUser.DB == constants.ExternalDB {
			continue
		}
		_, ok := m[user{db: mongoDBUser.DB, name: mongoDBUser.Name}]
		if !ok {
			deletedUsers = append(deletedUsers, automationconfig.DeletedUser{User: mongoDBUser.Name, Dbs: []string{mongoDBUser.DB}})
		}
	}
	return deletedUsers
}
