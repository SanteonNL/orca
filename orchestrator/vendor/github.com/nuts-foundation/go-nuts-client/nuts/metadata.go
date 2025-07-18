package nuts

import (
	ssi "github.com/nuts-foundation/go-did"
	"github.com/nuts-foundation/go-did/vc"
)

var _ CredentialProvider = EmployeeDetails{}

type EmployeeDetails struct {
	Id   string
	Name string
	Role string
}

func (e EmployeeDetails) Credentials() []vc.VerifiableCredential {
	return []vc.VerifiableCredential{
		{
			Context: []ssi.URI{
				ssi.MustParseURI("https://www.w3.org/2018/credentials/v1"),
				ssi.MustParseURI("https://nuts.nl/credentials/v1"),
			},
			Type: []ssi.URI{
				ssi.MustParseURI("VerifiableCredential"),
				ssi.MustParseURI("NutsEmployeeCredential"),
			},
			CredentialSubject: []map[string]interface{}{
				{
					"identifier": e.Id,
					"name":       e.Name,
					"roleName":   e.Role,
				},
			},
		},
	}
}
