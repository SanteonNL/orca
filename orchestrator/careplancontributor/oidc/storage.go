package oidc

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"sync"
	"time"
)

var _ op.Storage = (*Storage)(nil)
var _ op.CanSetUserinfoFromRequest = (*Storage)(nil)

const ScopePatient = "patient"
const ClaimPatient = "patient"
const ClaimRoles = "roles"

type Storage struct {
	mux          *sync.RWMutex
	authRequests map[string]AuthRequest
	tokens       map[string]Token
	clients      map[string]op.Client
	signingKey   SigningKey
}

func (o Storage) AuthenticateUser(ctx context.Context, authRequestID string, user UserDetails) error {
	o.mux.Lock()
	defer o.mux.Unlock()
	authRequest, ok := o.authRequests[authRequestID]
	if !ok {
		return errors.New("auth request not found")
	}
	err := authRequest.Authenticate(user)
	if err != nil {
		return err
	}
	o.authRequests[authRequestID] = authRequest
	return nil
}

func (o Storage) CreateAuthRequest(ctx context.Context, request *oidc.AuthRequest, _ string) (op.AuthRequest, error) {
	o.mux.Lock()
	defer o.mux.Unlock()
	authRequestID := uuid.NewString()
	req := AuthRequest{
		ID:          authRequestID,
		AuthRequest: *request,
	}
	o.authRequests[authRequestID] = req
	return &req, nil
}

func (o Storage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	o.mux.RLock()
	defer o.mux.RUnlock()
	req, ok := o.authRequests[id]
	if !ok {
		return nil, errors.New("auth request not found")
	}
	return &req, nil
}

func (o Storage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	o.mux.RLock()
	defer o.mux.RUnlock()
	for _, req := range o.authRequests {
		if req.Code == code {
			return &req, nil
		}
	}
	return nil, errors.New("auth request not found")
}

func (o Storage) SaveAuthCode(ctx context.Context, id string, code string) error {
	o.mux.Lock()
	defer o.mux.Unlock()
	req, ok := o.authRequests[id]
	if !ok {
		return errors.New("auth request not found")
	}
	req.Code = code
	o.authRequests[id] = req
	return nil
}

func (o Storage) DeleteAuthRequest(ctx context.Context, id string) error {
	o.mux.Lock()
	defer o.mux.Unlock()
	if _, ok := o.authRequests[id]; !ok {
		return errors.New("auth request not found")
	}
	delete(o.authRequests, id)
	return nil
}

func (o Storage) CreateAccessToken(ctx context.Context, request op.TokenRequest) (accessTokenID string, expiration time.Time, err error) {
	req, ok := request.(*AuthRequest)
	if !ok {
		return "", time.Time{}, fmt.Errorf("invalid token request: %T", request)
	}

	o.mux.Lock()
	defer o.mux.Unlock()
	token := &Token{
		ID:       uuid.NewString(),
		Audience: request.GetAudience(),
		Scopes:   request.GetScopes(),
		User:     *req.User,
	}
	o.tokens[token.ID] = *token
	return token.ID, time.Now().Add(5 * time.Minute), nil
}

func (o Storage) SigningKey(ctx context.Context) (op.SigningKey, error) {
	return o.signingKey, nil
}

func (o Storage) SignatureAlgorithms(ctx context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{o.signingKey.SignatureAlgorithm()}, nil
}

func (o Storage) KeySet(ctx context.Context) ([]op.Key, error) {
	return []op.Key{
		o.signingKey.Public(),
	}, nil
}

func (o Storage) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	o.mux.RLock()
	defer o.mux.RUnlock()
	client, ok := o.clients[clientID]
	if !ok {
		return nil, errors.New("client not found")
	}
	return client, nil
}

func (o Storage) AuthorizeClientIDSecret(ctx context.Context, clientID, clientSecret string) error {
	// TODO: Implement this
	return nil
}

// SetUserinfoFromScopes sets the userinfo claims based on the requested scopes and user ID.
// Since we don't want to store the userinfo in the database, we just return nil here.
// User info should then be set through SetUserinfoFromRequest
func (o Storage) SetUserinfoFromScopes(ctx context.Context, userinfo *oidc.UserInfo, userID, clientID string, scopes []string) error {
	return nil
}

func (o Storage) SetUserinfoFromRequest(ctx context.Context, userinfo *oidc.UserInfo, request op.IDTokenRequest, scopes []string) error {
	req, ok := request.(*AuthRequest)
	if !ok {
		return fmt.Errorf("only *AuthRequest is supported, but was: %T", request)
	}
	if req.User == nil || !req.AuthDone {
		return errors.New("user not authenticated")
	}
	populateUserInfo(userinfo, scopes, *req.User)
	return nil
}

func (o Storage) SetUserinfoFromToken(ctx context.Context, userInfo *oidc.UserInfo, tokenID, subject, origin string) error {
	o.mux.RLock()
	token, ok := o.tokens[tokenID]
	o.mux.RUnlock()
	if !ok {
		return errors.New("token not found")
	}
	populateUserInfo(userInfo, token.Scopes, token.User)
	return nil
}

func populateUserInfo(userInfo *oidc.UserInfo, scopes []string, user UserDetails) {
	userInfo.Claims = map[string]any{}
	for _, scope := range scopes {
		switch scope {
		case ScopePatient:
			var patientIdentifiers []string
			for _, identifier := range user.PatientIdentifiers {
				patientIdentifiers = append(patientIdentifiers, coolfhir.IdentifierToToken(identifier))
			}
			userInfo.Claims[ClaimPatient] = patientIdentifiers
		case oidc.ScopeOpenID:
			userInfo.Subject = user.ID
		case oidc.ScopeEmail:
			userInfo.Email = user.Email
		case oidc.ScopeProfile:
			userInfo.Name = user.Name
			userInfo.Claims[ClaimRoles] = user.Roles
		}
	}
}

func (o Storage) GetKeyByIDAndClientID(ctx context.Context, keyID, clientID string) (*jose.JSONWebKey, error) {
	//TODO implement me
	panic("GetKeyByIDAndClientID")
}

func (o Storage) ValidateJWTProfileScopes(ctx context.Context, userID string, scopes []string) ([]string, error) {
	//TODO implement me
	panic("ValidateJWTProfileScopes")
}

func (o Storage) GetPrivateClaimsFromScopes(ctx context.Context, userID, clientID string, scopes []string) (map[string]any, error) {
	// No private claims
	return nil, nil
}

func (o Storage) Health(ctx context.Context) error {
	// OK
	return nil
}

func (o Storage) SetIntrospectionFromToken(ctx context.Context, userinfo *oidc.IntrospectionResponse, tokenID, subject, clientID string) error {
	return errors.New("token introspection not supported")
}

func (o Storage) CreateAccessAndRefreshTokens(ctx context.Context, request op.TokenRequest, currentRefreshToken string) (accessTokenID string, newRefreshTokenID string, expiration time.Time, err error) {
	return "", "", time.Time{}, errors.New("refresh tokens not supported")
}

func (o Storage) TokenRequestByRefreshToken(ctx context.Context, refreshTokenID string) (op.RefreshTokenRequest, error) {
	return nil, errors.New("refresh tokens not supported")
}

func (o Storage) TerminateSession(ctx context.Context, userID string, clientID string) error {
	return errors.New("logout not supported")
}

func (o Storage) RevokeToken(ctx context.Context, tokenOrTokenID string, userID string, clientID string) *oidc.Error {
	return &oidc.Error{
		ErrorType:   "invalid_request",
		Description: "token revocation is not supported",
	}
}

func (o Storage) GetRefreshTokenInfo(ctx context.Context, clientID string, token string) (userID string, tokenID string, err error) {
	return "", "", errors.New("refresh tokens not supported")
}

type Token struct {
	ID       string
	Audience []string
	Scopes   []string
	User     UserDetails
}

type UserDetails struct {
	ID    string
	Name  string
	Email string
	Roles []string

	PatientIdentifiers []fhir.Identifier
}

type AuthRequest struct {
	oidc.AuthRequest
	ID string

	User          *UserDetails
	AuthTime      time.Time
	AuthDone      bool
	Code          string
	ApplicationID string
}

func (a *AuthRequest) Authenticate(details UserDetails) error {
	if a.AuthDone {
		return errors.New("already authenticated")
	}
	a.User = &details
	a.AuthDone = true
	a.AuthTime = time.Now()
	return nil
}

func (a AuthRequest) GetID() string {
	return a.ID
}

func (a AuthRequest) GetACR() string {
	return "TODO"
}

func (a AuthRequest) GetAMR() []string {
	return []string{"TODO"}
}

func (a AuthRequest) GetAudience() []string {
	return []string{a.ClientID}
}

func (a AuthRequest) GetAuthTime() time.Time {
	return a.AuthTime
}

func (a AuthRequest) GetClientID() string {
	return a.ClientID
}

func (a AuthRequest) GetCodeChallenge() *oidc.CodeChallenge {
	return &oidc.CodeChallenge{
		Challenge: a.CodeChallenge,
		Method:    a.CodeChallengeMethod,
	}
}

func (a AuthRequest) GetNonce() string {
	return a.Nonce
}

func (a AuthRequest) GetScopes() []string {
	return a.Scopes
}

func (a AuthRequest) GetSubject() string {
	if a.User == nil {
		return ""
	}
	return a.User.ID
}

func (a AuthRequest) Done() bool {
	return a.AuthDone
}
