module gates/discovery

// Discovery and the source of the keys that verify GATES OIDC JWTs, in three
// layers written without looking at each other: the standard layer
// (OpenID Connect Discovery, RFC 8725), the GitHub layer (Documented, then
// Published), and the GATES layer (GOJ-DISCOVERY-*).
// Values are decoded JSON values. A Text atom is one string after the JSON
// escaping is removed, so two strings are equal exactly when they are the same
// atom; the model has no escaping, normalization, or case folding.
// HTTP, TLS, and the bytes of the documents are outside the model, and so are
// the members of the keys, which the signature model covers.
// The scope of every command holds the one sig strings and the documents of a
// GitHub instance; the consistency runs show that the instances exist in it.

abstract sig Bool {}
one sig True, False extends Bool {}

// ---- Values ----

abstract sig Value {}
// A JSON value that is neither a string nor an array.
sig Other extends Value {}
// A JSON array. Only the values it contains matter in this model.
sig Array extends Value { elements: set Value }
abstract sig Text extends Value {}
// A string that the model does not read as a URL.
sig Word extends Text {}
one sig RS256, None, Code, IdToken, IdTokenToken, Public, Pairwise extends Word {}

abstract sig Scheme {}
one sig Https, OtherScheme extends Scheme {}
// A host with its port.
sig Host {}

// A string that parses as an absolute URL. configuration is the URL formed by
// appending /.well-known/openid-configuration to it.
sig URL extends Text {
  scheme: one Scheme,
  host: one Host,
  query: one Bool,
  fragment: one Bool,
  configuration: lone URL
}

fact Configuration {
  // Appending a path keeps the scheme and the host and gives another URL.
  all u: URL | some u.configuration implies u.configuration.scheme = u.scheme and u.configuration.host = u.host
  configuration.~configuration in iden
  no u: URL | u in u.^configuration
}

// https://token.actions.githubusercontent.com, the Issuer that GOJ-ISS fixes,
// and https://token.actions.githubusercontent.com/.well-known/jwks. The facts
// are what the two strings are as URLs.
one sig ActionsIssuer, ActionsJwks extends URL {}

fact ActionsURLs {
  ActionsIssuer.scheme = Https and ActionsIssuer.query = False and ActionsIssuer.fragment = False
  some ActionsIssuer.configuration
  ActionsJwks.scheme = Https and ActionsJwks.host = ActionsIssuer.host
  ActionsJwks.query = False and ActionsJwks.fragment = False
  ActionsJwks not in ActionsIssuer + ActionsIssuer.configuration
}

// ---- Documents and tokens ----

// A public key that verifies signatures.
sig Key {}

// A JWK Set document, by the keys it holds.
sig KeySet { keys: set Key }

// An OpenID Provider configuration document. json is False when the body is
// not a JSON object; such a document has no members.
sig Discovery {
  json: one Bool,
  issuer: lone Value,
  jwksUri: lone Value,
  authorizationEndpoint: lone Value,
  tokenEndpoint: lone Value,
  responseTypes: lone Value,
  subjectTypes: lone Value,
  signingAlgs: lone Value,
  // The document has members other than those above.
  additional: one Bool
}

fact NonJSONDiscovery {
  all d: Discovery | d.json = False implies {
    no d.(issuer + jwksUri + authorizationEndpoint + tokenEndpoint + responseTypes + subjectTypes + signingAlgs)
    d.additional = False
  }
}

// What each URL returns at the time of the retrieval.
one sig Web { document: URL -> lone (Discovery + KeySet) }

fun discoveryAt[u: Value]: lone Discovery { (u & URL).(Web.document) & Discovery }
fun keySetAt[u: Value]: lone KeySet { (u & URL).(Web.document) & KeySet }

// A JWT, by its iss, its alg, and the key whose private key signed it.
sig Token {
  iss: lone Value,
  alg: lone Value,
  key: lone Key
}

// ---- Standard layer: OpenID Connect Discovery 1.0 ----

// trace:formalizes OIDCD-ISSUER-REQUIRED
pred stdIssuerRequired[d: Discovery] { some d.issuer }

// trace:formalizes OIDCD-ISSUER-FORMAT
pred stdIssuerFormat[d: Discovery] {
  some d.issuer implies (d.issuer in URL and d.issuer.scheme = Https and d.issuer.query = False and d.issuer.fragment = False)
}

// trace:unformalized OIDCD-ISSUER-WEBFINGER モデルは WebFinger による Issuer の発見を表さない

// idToken is the Condition: t is an ID Token issued from the Issuer of d.
// trace:formalizes OIDCD-ISSUER-ID-TOKEN-ISS
pred stdIssuerIdTokenIss[d: Discovery, t: Token, idToken: Bool] {
  idToken = True implies (some t.iss and t.iss = d.issuer)
}

// trace:formalizes OIDCD-AUTHORIZATION-ENDPOINT-REQUIRED
pred stdAuthorizationEndpointRequired[d: Discovery] { some d.authorizationEndpoint }

// trace:formalizes OIDCD-AUTHORIZATION-ENDPOINT-HTTPS
pred stdAuthorizationEndpointHttps[d: Discovery] {
  some d.authorizationEndpoint implies (d.authorizationEndpoint in URL and d.authorizationEndpoint.scheme = Https)
}

// The choice the MAY permits. The model has no port or path of a URL.
// trace:formalizes OIDCD-AUTHORIZATION-ENDPOINT-COMPONENTS
pred stdAuthorizationEndpointQuery[d: Discovery] { d.authorizationEndpoint.query = True }

// implicitOnly negates the Condition: only the Implicit Flow is used.
// trace:formalizes OIDCD-TOKEN-ENDPOINT-REQUIRED
pred stdTokenEndpointRequired[d: Discovery, implicitOnly: Bool] {
  implicitOnly = False implies some d.tokenEndpoint
}

// trace:formalizes OIDCD-TOKEN-ENDPOINT-HTTPS
pred stdTokenEndpointHttps[d: Discovery] {
  some d.tokenEndpoint implies (d.tokenEndpoint in URL and d.tokenEndpoint.scheme = Https)
}

// trace:formalizes OIDCD-TOKEN-ENDPOINT-COMPONENTS
pred stdTokenEndpointQuery[d: Discovery] { d.tokenEndpoint.query = True }

// trace:formalizes OIDCD-JWKS-URI-REQUIRED
pred stdJwksUriRequired[d: Discovery] { some d.jwksUri }

// trace:formalizes OIDCD-JWKS-URI-HTTPS
pred stdJwksUriHttps[d: Discovery] {
  some d.jwksUri implies (d.jwksUri in URL and d.jwksUri.scheme = Https)
}

// trace:formalizes OIDCD-RESPONSE-TYPES-REQUIRED
pred stdResponseTypesRequired[d: Discovery] { d.responseTypes in Array and some d.responseTypes }

// trace:formalizes OIDCD-RESPONSE-TYPES-DYNAMIC
pred stdResponseTypesDynamic[d: Discovery, dynamic: Bool] {
  dynamic = True implies Code + IdToken + IdTokenToken in d.responseTypes.elements
}

// trace:formalizes OIDCD-SUBJECT-TYPES-REQUIRED
pred stdSubjectTypesRequired[d: Discovery] { d.subjectTypes in Array and some d.subjectTypes }

// trace:formalizes OIDCD-ID-TOKEN-SIGNING-ALG-REQUIRED
pred stdSigningAlgsRequired[d: Discovery] { d.signingAlgs in Array and some d.signingAlgs }

// trace:formalizes OIDCD-ID-TOKEN-SIGNING-ALG-RS256
pred stdSigningAlgsRS256[d: Discovery] { RS256 in d.signingAlgs.elements }

// The choice the MAY permits.
// trace:formalizes OIDCD-ID-TOKEN-SIGNING-ALG-NONE-SUPPORT
pred stdSigningAlgsNone[d: Discovery] { None in d.signingAlgs.elements }

// fromAuthorizationEndpoint is the part of the Condition that the Response
// Type returns an ID Token from the Authorization Endpoint.
// trace:formalizes OIDCD-ID-TOKEN-SIGNING-ALG-NONE-USE
pred stdNoneUse[t: Token, idToken: Bool, fromAuthorizationEndpoint: Bool] {
  (idToken = True and fromAuthorizationEndpoint = True) implies t.alg != None
}

// The choice the two MAY clauses permit.
// trace:formalizes OIDCD-METADATA-ADDITIONAL OIDCD-CONFIG-OTHER-CLAIMS
pred stdAdditionalMembers[d: Discovery] { d.additional = True }

// supported is the Condition: the OpenID Provider of Issuer i supports Discovery.
// trace:formalizes OIDCD-CONFIG-PATH
pred stdConfigPath[i: URL, supported: Bool] {
  supported = True implies (some discoveryAt[i.configuration] and discoveryAt[i.configuration].json = True)
}

// supported is the Condition. Being compliant with the specification is the
// other clauses on the Metadata; this predicate is the part that the document
// is JSON.
// trace:formalizes OIDCD-CONFIG-DOCUMENT
pred stdConfigDocument[d: Discovery, supported: Bool] { supported = True implies d.json = True }

// trace:unformalized OIDCD-CONFIG-CONTENT-TYPE モデルは HTTP の応答のヘッダーを扱わない
// trace:unformalized OIDCD-CONFIG-GET モデルは HTTP の要求のメソッドを扱わない
// trace:unformalized OIDCD-CONFIG-PATH-SLASH モデルは URL のパスを文字列として表さず、末尾の / の除去を表さない
// trace:unformalized OIDCD-CONFIG-RESPONSE モデルは HTTP の応答のステータスとヘッダーを扱わない

// trace:formalizes OIDCD-CONFIG-EMPTY-OMIT
pred stdConfigEmptyOmit[d: Discovery] {
  all a: d.(responseTypes + subjectTypes + signingAlgs) & Array | some a.elements
}

// The validation of OpenID Connect Discovery that the model has: the issuer
// matches the Issuer URL used to retrieve the configuration. The TLS
// certificate check is outside the model.
// trace:formalizes OIDCD-CONFIG-ISSUER-MATCH
pred stdConfigIssuerMatch[d: Discovery, prefix: URL] { d.issuer = prefix }

// completed tells whether an operation that requires the information of d,
// retrieved with the Issuer URL prefix, completed.
// trace:formalizes OIDCD-CONFIG-VALIDATION-ABORT
pred stdConfigValidationAbort[d: Discovery, prefix: URL, completed: Bool] {
  not stdConfigIssuerMatch[d, prefix] implies completed = False
}

// used is the information of d that the Relying Party uses.
// trace:formalizes OIDCD-CONFIG-VALIDATION-UNUSED
pred stdConfigValidationUnused[d: Discovery, prefix: URL, used: set Value] {
  not stdConfigIssuerMatch[d, prefix] implies no used
}

// trace:formalizes OIDCD-CONFIG-ISSUER-ID-TOKEN
pred stdConfigIssuerIdToken[d: Discovery, t: Token, idToken: Bool, accepted: Bool] {
  (idToken = True and accepted = True) implies (some t.iss and t.iss = d.issuer)
}

// trace:unformalized OIDCD-STRING-UNESCAPE モデルの文字列は JSON のエスケープを除いた後の値で、エスケープの除去を表さない

// A string is its sequence of code points, so the comparison of two strings is
// the identity of their atoms, and no normalization applies.
// trace:formalizes OIDCD-STRING-CODE-POINT OIDCD-STRING-NO-NORMALIZATION
pred stdStringEqual[a, b: Value] { one a and a in Text and a = b }

// trace:unformalized OIDCD-TLS-SUPPORT モデルは TLS を扱わない
// trace:unformalized OIDCD-TLS-BCP195 モデルは TLS を扱わない
// trace:unformalized OIDCD-TLS-CONFIDENTIALITY モデルは TLS を扱わない
// trace:unformalized OIDCD-TLS-CERTIFICATE モデルは TLS を扱わない
// trace:unformalized OIDCD-CONFIG-TLS-CHECK モデルは TLS を扱わない

// ---- Standard layer: RFC 8725 ----

// belongs tells which keys belong to which Issuer; how it is determined is
// specific to the application.
// trace:formalizes RFC8725-ISS-KEY-VALIDATE
pred stdIssKeyValidate[t: Token, accepted: Bool, belongs: Text -> Key] {
  (accepted = True and some t.iss) implies (some t.key and t.key in belongs[t.iss])
}

// The way of determining the keys of an Issuer that RFC 8725 §3.8 gives as an
// example: the JWK Set at the jwks_uri of the metadata of the Issuer's https URL.
fun metadataKeys: Text -> Key {
  { i: URL, k: Key | i.scheme = Https and some d: discoveryAt[i.configuration] | d.issuer = i and k in keySetAt[d.jwksUri].keys }
}

// ---- GitHub layer: Documented ----

// trace:formalizes GH-OIDC-ISS
pred ghIss[t: Token] { some t.iss implies t.iss = ActionsIssuer }

// trace:formalizes GH-OIDC-DISCOVERY-LOCATION
pred ghDiscoveryLocation { some d: discoveryAt[ActionsIssuer.configuration] | d.json = True }

pred ghDocumented[t: Token] { ghIss[t] and ghDiscoveryLocation }

// ---- GitHub layer: Published ----

fun ghDiscovery: lone Discovery { discoveryAt[ActionsIssuer.configuration] }

// trace:formalizes GH-OIDC-DISCOVERY-ISSUER
pred ghDiscoveryIssuer { ghDiscovery.json = True and ghDiscovery.issuer = ActionsIssuer }

// trace:formalizes GH-OIDC-DISCOVERY-JWKS-URI
pred ghDiscoveryJwksUri { ghDiscovery.json = True and ghDiscovery.jwksUri = ActionsJwks }

// The model has no claims_supported or scopes_supported, which are additional
// members here.
// trace:formalizes GH-OIDC-DISCOVERY-MEMBERS
pred ghDiscoveryMembers {
  let d = ghDiscovery {
    d.json = True
    some d.issuer and some d.jwksUri and some d.signingAlgs
    d.subjectTypes in Array and d.subjectTypes.elements = Public + Pairwise
    d.responseTypes in Array and d.responseTypes.elements = IdToken
    d.additional = True
    no d.authorizationEndpoint and no d.tokenEndpoint
  }
}

// trace:formalizes GH-OIDC-DISCOVERY-ALG
pred ghDiscoveryAlg { ghDiscovery.signingAlgs in Array and ghDiscovery.signingAlgs.elements = RS256 }

pred ghPublished[t: Token] {
  ghDocumented[t]
  ghDiscoveryIssuer and ghDiscoveryJwksUri and ghDiscoveryMembers and ghDiscoveryAlg
}

// ---- GATES layer ----

// trace:formalizes GOJ-DISCOVERY-ISSUER
pred gatesDiscoveryIssuer[d: Discovery] {
  d = discoveryAt[ActionsIssuer.configuration]
  d.json = True
  stdStringEqual[d.issuer, ActionsIssuer]
}

// The host includes the port.
// trace:formalizes GOJ-DISCOVERY-JWKS-URI
pred gatesDiscoveryJwksUri[d: Discovery] {
  gatesDiscoveryIssuer[d]
  d.jwksUri in URL and some d.jwksUri
  d.jwksUri.scheme = Https
  d.jwksUri.host = ActionsIssuer.host
}

// The metadata GATES uses from a Discovery document.
fun gatesUsedMetadata[d: Discovery]: set Value { gatesDiscoveryIssuer[d] => d.jwksUri else none }

// GATES accepts t only with a key of the key set at the jwks_uri it accepted,
// and only when iss is the fixed Issuer (GOJ-ISS, which the Z3 model formalizes).
pred gatesKeySource[t: Token, d: Discovery, s: KeySet] {
  gatesDiscoveryJwksUri[d]
  s = keySetAt[gatesUsedMetadata[d]]
  t.key in s.keys and some t.key
  t.iss = ActionsIssuer
}

pred gatesAccepts[t: Token] { some d: Discovery, s: KeySet | gatesKeySource[t, d, s] }

// ---- Mutations of the GATES layer ----

// Uses a Discovery document without matching its issuer.
pred mutantKeySourceWithoutIssuerMatch[t: Token, d: Discovery, s: KeySet] {
  d = discoveryAt[ActionsIssuer.configuration] and d.json = True
  s = keySetAt[d.jwksUri]
  t.key in s.keys and some t.key
  t.iss = ActionsIssuer
}

// Uses any retrieved key set, not the one at the jwks_uri.
pred mutantKeySourceAnyKeySet[t: Token, s: KeySet] {
  some d: Discovery | gatesDiscoveryJwksUri[d]
  s in URL.(Web.document)
  t.key in s.keys and some t.key
  t.iss = ActionsIssuer
}

// ---- Consistency ----

pred stdOpenIDProvider[i: URL, d: Discovery] {
  stdConfigPath[i, True] and d = discoveryAt[i.configuration]
  stdConfigDocument[d, True] and stdConfigIssuerMatch[d, i]
  stdIssuerRequired[d] and stdIssuerFormat[d]
  stdAuthorizationEndpointRequired[d] and stdAuthorizationEndpointHttps[d]
  stdTokenEndpointRequired[d, False] and stdTokenEndpointHttps[d]
  stdJwksUriRequired[d] and stdJwksUriHttps[d]
  stdResponseTypesRequired[d] and stdSubjectTypesRequired[d]
  stdSigningAlgsRequired[d] and stdSigningAlgsRS256[d]
  stdConfigEmptyOmit[d]
}

pred StandardLayerConsistent {
  some i: URL, d: Discovery, t: Token {
    stdOpenIDProvider[i, d]
    stdResponseTypesDynamic[d, True] and stdSigningAlgsNone[d] and stdAdditionalMembers[d]
    stdAuthorizationEndpointQuery[d] and stdTokenEndpointQuery[d]
    stdIssuerIdTokenIss[d, t, True] and stdNoneUse[t, True, True]
    stdConfigValidationAbort[d, i, True] and stdConfigValidationUnused[d, i, d.jwksUri]
    stdConfigIssuerIdToken[d, t, True, True]
    stdIssKeyValidate[t, True, metadataKeys] and some t.key
  }
}

pred GitHubDocumentedConsistent { some t: Token | ghDocumented[t] and some t.iss }

pred GitHubPublishedConsistent { some t: Token | ghPublished[t] and some t.iss }

pred GatesLayerConsistent { some t: Token | gatesAccepts[t] }

run StandardLayerConsistent for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 1
// trace:checks GH-OIDC-ISS GH-OIDC-DISCOVERY-LOCATION
run GitHubDocumentedConsistent for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 1
// trace:checks GH-OIDC-DISCOVERY-ISSUER GH-OIDC-DISCOVERY-JWKS-URI GH-OIDC-DISCOVERY-MEMBERS GH-OIDC-DISCOVERY-ALG
run GitHubPublishedConsistent for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 1
// trace:checks GOJ-DISCOVERY-ISSUER GOJ-DISCOVERY-JWKS-URI
run GatesLayerConsistent for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 1

// ---- Safety: A satisfies S ----

assert DiscoveryIssuerMatchesIssuerURL {
  all d: Discovery | gatesDiscoveryIssuer[d] implies stdConfigIssuerMatch[d, ActionsIssuer]
}

assert AcceptanceNeedsValidatedDiscovery {
  all t: Token, d: Discovery, s: KeySet |
    gatesKeySource[t, d, s] implies stdConfigValidationAbort[d, ActionsIssuer, True]
}

assert UnvalidatedDiscoveryIsUnused {
  all d: Discovery | stdConfigValidationUnused[d, ActionsIssuer, gatesUsedMetadata[d]]
}

assert DiscoveryIssuerComparedByCodePoint {
  all d: Discovery | gatesDiscoveryIssuer[d] implies stdStringEqual[d.issuer, ActionsIssuer]
}

assert JwksUriIsHttps {
  all d: Discovery | gatesDiscoveryJwksUri[d] implies stdJwksUriHttps[d]
}

// GOJ-ISS fixes iss, and the key comes from the jwks_uri of the Discovery of
// that Issuer, which is how RFC 8725 §3.8 determines the keys of the Issuer.
assert KeyBelongsToIssuer {
  all t: Token | gatesAccepts[t] implies stdIssKeyValidate[t, True, metadataKeys]
}

// trace:checks GOJ-DISCOVERY-ISSUER OIDCD-CONFIG-ISSUER-MATCH
check DiscoveryIssuerMatchesIssuerURL for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 0
// trace:checks GOJ-DISCOVERY-ISSUER OIDCD-CONFIG-VALIDATION-ABORT
check AcceptanceNeedsValidatedDiscovery for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 0
// trace:checks GOJ-DISCOVERY-ISSUER OIDCD-CONFIG-VALIDATION-UNUSED
check UnvalidatedDiscoveryIsUnused for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 0
// trace:checks GOJ-DISCOVERY-ISSUER OIDCD-STRING-CODE-POINT OIDCD-STRING-NO-NORMALIZATION
check DiscoveryIssuerComparedByCodePoint for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 0
// trace:checks GOJ-DISCOVERY-JWKS-URI OIDCD-JWKS-URI-HTTPS
check JwksUriIsHttps for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 0
// trace:checks GOJ-DISCOVERY-JWKS-URI RFC8725-ISS-KEY-VALIDATE
check KeyBelongsToIssuer for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 0

// ---- Properties of the GATES layer ----

// GATES never uses a key outside the JWK Set of the fixed Issuer.
assert KeysOnlyFromFixedIssuer {
  all t: Token, d: Discovery, s: KeySet | gatesKeySource[t, d, s] implies
    s = keySetAt[discoveryAt[ActionsIssuer.configuration].jwksUri] and
    discoveryAt[ActionsIssuer.configuration].issuer = ActionsIssuer
}

// trace:checks GOJ-DISCOVERY-ISSUER GOJ-DISCOVERY-JWKS-URI
check KeysOnlyFromFixedIssuer for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 0

// ---- Mutations: the checks find a wrong GATES layer ----

assert MutantWithoutIssuerMatchMatchesIssuerURL {
  all t: Token, d: Discovery, s: KeySet |
    mutantKeySourceWithoutIssuerMatch[t, d, s] implies stdConfigIssuerMatch[d, ActionsIssuer]
}

assert MutantAnyKeySetKeepsKeyOfIssuer {
  all t: Token, s: KeySet | mutantKeySourceAnyKeySet[t, s] implies stdIssKeyValidate[t, True, metadataKeys]
}

check MutantWithoutIssuerMatchMatchesIssuerURL for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 1
check MutantAnyKeySetKeepsKeyOfIssuer for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 1

// ---- Compatibility: G is accepted by A ----

assert GitHubDiscoveryAccepted {
  all t: Token | (ghPublished[t] and some t.iss) implies
    (t.iss = ActionsIssuer and some d: Discovery | gatesDiscoveryIssuer[d])
}

assert GitHubJwksUriAccepted {
  all t: Token | ghPublished[t] implies some d: Discovery | gatesDiscoveryJwksUri[d]
}

// The Documented clauses alone do not settle the issuer of the Discovery document.
pred DiscoveryIssuerNeedsPublished {
  some t: Token | ghDocumented[t] and some t.iss and no d: Discovery | gatesDiscoveryIssuer[d]
}

// The Documented clauses alone do not settle the jwks_uri.
pred JwksUriNeedsPublished {
  some t: Token | ghDocumented[t] and ghDiscoveryIssuer and no d: Discovery | gatesDiscoveryJwksUri[d]
}

// trace:checks GOJ-DISCOVERY-ISSUER GH-OIDC-DISCOVERY-ISSUER GH-OIDC-DISCOVERY-LOCATION GH-OIDC-ISS
check GitHubDiscoveryAccepted for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 0
// trace:checks GOJ-DISCOVERY-JWKS-URI GH-OIDC-DISCOVERY-JWKS-URI
check GitHubJwksUriAccepted for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 0
// trace:checks GOJ-DISCOVERY-ISSUER
run DiscoveryIssuerNeedsPublished for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 1
// trace:checks GOJ-DISCOVERY-JWKS-URI
run JwksUriNeedsPublished for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 1

// ---- Examples: the Published Discovery document and the standard layer ----

// The Published metadata satisfies these clauses for an OpenID Provider.
assert GitHubDiscoveryMeetsStandardMetadata {
  all t: Token | ghPublished[t] implies {
    stdConfigPath[ActionsIssuer, True]
    let d = ghDiscovery {
      stdConfigDocument[d, True] and stdConfigIssuerMatch[d, ActionsIssuer]
      stdIssuerRequired[d] and stdIssuerFormat[d]
      stdJwksUriRequired[d] and stdJwksUriHttps[d]
      stdResponseTypesRequired[d] and stdSubjectTypesRequired[d]
      stdSigningAlgsRequired[d] and stdSigningAlgsRS256[d]
      stdConfigEmptyOmit[d]
    }
  }
}

// The Published metadata has no authorization_endpoint or token_endpoint.
assert GitHubDiscoveryHasEndpoints {
  all t: Token | ghPublished[t] implies
    (stdAuthorizationEndpointRequired[ghDiscovery] or stdTokenEndpointRequired[ghDiscovery, False])
}

// trace:checks GH-OIDC-DISCOVERY-ISSUER GH-OIDC-DISCOVERY-JWKS-URI GH-OIDC-DISCOVERY-MEMBERS GH-OIDC-DISCOVERY-ALG OIDCD-CONFIG-PATH OIDCD-CONFIG-DOCUMENT OIDCD-CONFIG-ISSUER-MATCH OIDCD-ISSUER-REQUIRED OIDCD-ISSUER-FORMAT OIDCD-JWKS-URI-REQUIRED OIDCD-JWKS-URI-HTTPS OIDCD-RESPONSE-TYPES-REQUIRED OIDCD-SUBJECT-TYPES-REQUIRED OIDCD-ID-TOKEN-SIGNING-ALG-REQUIRED OIDCD-ID-TOKEN-SIGNING-ALG-RS256 OIDCD-CONFIG-EMPTY-OMIT
check GitHubDiscoveryMeetsStandardMetadata for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 0
// trace:checks GH-OIDC-DISCOVERY-MEMBERS OIDCD-AUTHORIZATION-ENDPOINT-REQUIRED OIDCD-TOKEN-ENDPOINT-REQUIRED
check GitHubDiscoveryHasEndpoints for 14 but 2 Discovery, 2 KeySet, 2 Token, 3 Key, 2 Host expect 1
