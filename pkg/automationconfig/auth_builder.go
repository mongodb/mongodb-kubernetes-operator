package automationconfig

type AuthBuilder struct {
	disabled, authoritativeSet bool
	users                      []MongoDBUser
	autoAuthMechanisms         []string
	autoAuthMechanism          string
	deploymentAuthMechanisms   []string
	autoUser                   string
	key                        string
	keyFile                    string
	keyFileWindows             string
	autoPwd                    string
}

func NewAuthBuilder() *AuthBuilder {
	return &AuthBuilder{
		users:                    []MongoDBUser{},
		autoAuthMechanisms:       []string{},
		deploymentAuthMechanisms: []string{},
	}
}

func (ab *AuthBuilder) Build() Auth {
	return Auth{
		Users:                    ab.users,
		Disabled:                 ab.disabled,
		AuthoritativeSet:         ab.authoritativeSet,
		AutoAuthMechanisms:       ab.autoAuthMechanisms,
		AutoAuthMechanism:        ab.autoAuthMechanism,
		DeploymentAuthMechanisms: ab.deploymentAuthMechanisms,
		AutoUser:                 ab.autoUser,
		Key:                      ab.key,
		KeyFile:                  ab.keyFile,
		KeyFileWindows:           ab.keyFileWindows,
		AutoPwd:                  ab.autoPwd,
	}
}

func (ab *AuthBuilder) SetDisabled(disabled bool) *AuthBuilder {
	ab.disabled = disabled
	return ab
}

func (ab *AuthBuilder) SetAuthoritativeSet(authoritativeSet bool) *AuthBuilder {
	ab.authoritativeSet = authoritativeSet
	return ab
}

func (ab *AuthBuilder) SetUsers(authoritativeSet bool) *AuthBuilder {
	ab.authoritativeSet = authoritativeSet
	return ab
}

//func (ab *AuthBuilder) AuthoritativeSet(authoritativeSet bool) *AuthBuilder {
//	ab.authoritativeSet = authoritativeSet
//	return ab
//}
//
//func (ab *AuthBuilder) AuthoritativeSet(authoritativeSet bool) *AuthBuilder {
//	ab.authoritativeSet = authoritativeSet
//	return ab
//}
//
//func (ab *AuthBuilder) AuthoritativeSet(authoritativeSet bool) *AuthBuilder {
//	ab.authoritativeSet = authoritativeSet
//	return ab
//}
