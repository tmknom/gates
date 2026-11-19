module gates/signature

// The JWK Set, the selection of a key by kid, and the signature verification of
// GATES OIDC JWTs, in three layers written without looking at each other: the
// standard layer (RFC 7515, RFC 7517, RFC 7518, RFC 8725, RFC 9964,
// OpenID Connect Core and Discovery), the GitHub layer (Published and
// Observed), and the GATES layer (GOJ-JWK-*, GOJ-X5C-*, GOJ-JWKS-*,
// GOJ-SIGNATURE).
// Values are decoded JSON values. A Text atom is one string after the JSON
// escaping is removed, so two strings are equal exactly when they are the same
// atom. What a string decodes to (a Base64urlUInt, a certificate, a URL) is a
// field of the string. A signature is modeled by who made it: it validates
// under a public key and an algorithm exactly when it was made with that
// algorithm and the private key of that public key. The bytes of JSON,
// Base64url, and the arithmetic of RSA are outside the model, and so are the
// retrieval of the documents and its retry, which the Go tests cover.
// The scope of every command holds the one sig strings and the JWK Set of a
// GitHub instance; the consistency runs show that the instances exist in it.
// The instances may use a one sig string as a kid, an n, or a thumbprint, so
// the scope does not hold a separate string for each of them.

abstract sig Bool {}
one sig True, False extends Bool {}

// ---- Values ----

abstract sig Value {}
// A JSON value that is neither a string nor an array, such as a number or an object.
sig Other extends Value {}
sig Array extends Value { elements: seq Value }
abstract sig Text extends Value {
  // The value of the string read as a Base64urlUInt, if it is one.
  uint: lone UInt,
  // The Base64urlUInt uses the minimum number of octets.
  minimal: one Bool,
  // The X.509 certificate of the string read as Base64 DER, if it is one.
  certificate: lone Certificate,
  // The string is the Base64url encoding of 20 octets, or of 32 octets.
  sha1Form: one Bool,
  sha256Form: one Bool,
  // The string parses as a URL.
  url: one Bool
}
one sig Empty, RSA, EC, Oct, AKP, Sig, Enc, Sign, Verify, Encrypt, Decrypt, WrapKey, UnwrapKey,
        RS256, PS256, ES256, HS256, None, AQAB extends Text {}
sig Word extends Text {}

// deriveKey and deriveBits, the other key operations RFC 7517 §4.3 defines.
sig DeriveOp in Word {}
// alg values by the kind of algorithm they name.
sig PkcsAlg, PssAlg, EcdsaAlg, MacAlg in Text {}

fun values[v: Value]: set Value { Int.(v.elements) }
pred repeated[v: Value] { some disj i, k: (v.elements).Value | (v.elements)[i] = (v.elements)[k] }

// A non-negative integer, by the properties the rules use.
sig UInt {
  odd: one Bool,
  // At least 2^2047, that is, 2048 bits or more.
  bits2048: one Bool,
  atLeast3: one Bool,
  // At most 2^31 − 1.
  atMost2p31: one Bool
}

fact Texts {
  all t: Text | t.minimal = True implies some t.uint
  all disj a, b: Text | (a.minimal = True and b.minimal = True) implies a.uint != b.uint
  no Empty.uint
  all u: UInt | u.bits2048 = True implies (u.atLeast3 = True and u.atMost2p31 = False)
  // AQAB is 65537.
  AQAB.minimal = True
  AQAB.uint.odd = True and AQAB.uint.atLeast3 = True and AQAB.uint.atMost2p31 = True and AQAB.uint.bits2048 = False
  RS256 in PkcsAlg and PS256 in PssAlg and ES256 in EcdsaAlg and HS256 in MacAlg
  no PkcsAlg & PssAlg and no PkcsAlg & EcdsaAlg and no PkcsAlg & MacAlg
  no PssAlg & EcdsaAlg and no PssAlg & MacAlg and no EcdsaAlg & MacAlg
  None not in PkcsAlg + PssAlg + EcdsaAlg + MacAlg
}

// ---- Keys and certificates ----

sig PublicKey {}
sig RsaKey extends PublicKey { modulus: one UInt, exponent: one UInt }

abstract sig KeyUsage {}
one sig DigitalSignature, KeyEncipherment, KeyCertSign extends KeyUsage {}

sig Certificate {
  // The public key of the subject.
  key: one PublicKey,
  keyUsageExtension: one Bool,
  keyUsage: set KeyUsage,
  // The public keys under which the signature of the certificate validates.
  signedBy: set PublicKey,
  // The Base64url encodings of the SHA-1 and SHA-256 thumbprints.
  sha1: one Text,
  sha256: one Text
}

fact Certificates {
  all disj p, q: RsaKey | p.modulus != q.modulus or p.exponent != q.exponent
  all c: Certificate | c.keyUsageExtension = False implies no c.keyUsage
  all c: Certificate | c.sha1.sha1Form = True and c.sha256.sha256Form = True
}

// ---- JWK and JWK Set ----

// The members that hold private or symmetric key values: k, d, p, q, dp, dq, qi, oth, priv.
abstract sig SecretMember {}
one sig K, D, P, Q, DP, DQ, QI, Oth, Priv extends SecretMember {}

// A member name that the model does not interpret.
sig Name {}

// An element of the keys array: a JSON object read as a JWK, or another value.
abstract sig Element {}
sig NonObject extends Element {}

// A JWK. A member that appears more than once has all its values in kty, and
// at most one value in the other members, because a JWK with a duplicate name
// matters only by whether it holds a secret value.
sig JWK extends Element {
  duplicateName: one Bool,
  kty: set Value,
  kid: lone Value,
  use: lone Value,
  keyOps: lone Value,
  alg: lone Value,
  n: lone Value,
  e: lone Value,
  pub: lone Value,
  x5c: lone Value,
  x5t: lone Value,
  x5tS256: lone Value,
  x5u: lone Value,
  secret: set SecretMember,
  unknown: set Name
}

fact JWKs {
  all j: JWK | j.duplicateName = False implies lone j.kty
  all j: JWK | (one j.n.uint and one j.e.uint) implies some keyOf[j]
}

// The RSA public key of the n and e of j.
fun keyOf[j: JWK]: lone RsaKey { { p: RsaKey | p.modulus = j.n.uint and p.exponent = j.e.uint } }

fun leaf[j: JWK]: lone Certificate { ((j.x5c.elements)[0] & Text).certificate }

// A JWK Set document. keysArray is False when keys is missing or not an array.
sig KeySet {
  json: one Bool,
  duplicateName: one Bool,
  keysArray: one Bool,
  elements: seq Element,
  unknown: set Name
}

fact KeySets {
  all s: KeySet | s.keysArray = False implies no s.elements
  all s: KeySet | s.json = False implies (s.keysArray = False and s.duplicateName = False and no s.unknown)
}

fun items[s: KeySet]: set Element { Int.(s.elements) }
fun jwks[s: KeySet]: set JWK { items[s] & JWK }

// ---- JWT and verification ----

sig JWT {
  // The kid and alg of the JOSE Header.
  kid: lone Value,
  alg: lone Value,
  // The public key of the private key that made the signature, and the
  // algorithm that made it. A forged signature has neither, and an Unsecured
  // JWS has the algorithm none and no signer.
  signer: lone PublicKey,
  signingAlg: lone Text,
  // The JWS Signature is the empty octet sequence.
  emptySignature: one Bool,
  // The payload is a JWT, and the cryptographic operations of that JWT
  // validate with the keys and algorithms the application supplies.
  nested: one Bool,
  innerValid: one Bool
}

fact JWTs {
  all t: JWT | some t.signer iff (some t.signingAlg and t.signingAlg != None)
  all t: JWT | t.emptySignature = True implies no t.signer
}

// A signature with an algorithm other than none validates under the public key
// whose private key made it; the empty signature of none validates without a key.
pred validates[t: JWT, p: PublicKey, a: Text] {
  t.signingAlg = a
  a = None => (no p and t.emptySignature = True) else (one p and t.signer = p)
}

abstract sig Outcome {}
one sig Accepted, AuthenticationFailure, InternalFailure extends Outcome {}

// One verification of a JWT with a JWK Set: the outcome, and the JWK and the
// algorithm of the cryptographic operation, if one was performed.
sig Verification {
  keySet: one KeySet,
  jwt: one JWT,
  outcome: one Outcome,
  key: lone JWK,
  opAlg: lone Text
}

// ---- Standard layer: RFC 7517 JWK ----

// trace:unformalized RFC7517-JWK-WHITESPACE モデルはデコード後の値を扱い、JSON のテキストの空白を扱わない

// trace:formalizes RFC7517-MEMBER-NAME-UNIQUE
pred stdJwkMemberNameUnique[j: JWK] { j.duplicateName = False }

// used tells whether the JWK Parser uses j, and lastWins whether its JSON
// parser returns only the lexically last of duplicate member names.
// trace:formalizes RFC7517-MEMBER-NAME-DUPLICATE
pred stdJwkMemberNameDuplicate[j: JWK, used: Bool, lastWins: Bool] {
  (j.duplicateName = True and used = True) implies lastWins = True
}

// The members the model does not interpret are the members the implementation
// does not understand.
pred sameExceptUnknown[j1, j2: JWK] {
  j1.duplicateName = j2.duplicateName and j1.kty = j2.kty and j1.kid = j2.kid
  j1.use = j2.use and j1.keyOps = j2.keyOps and j1.alg = j2.alg
  j1.n = j2.n and j1.e = j2.e and j1.pub = j2.pub
  j1.x5c = j2.x5c and j1.x5t = j2.x5t and j1.x5tS256 = j2.x5tS256 and j1.x5u = j2.x5u
  j1.secret = j2.secret
}

// used is the set of JWKs the implementation uses.
// trace:formalizes RFC7517-MEMBER-UNKNOWN-IGNORE
pred stdJwkMemberUnknownIgnore[used: set JWK] {
  all j1, j2: JWK | sameExceptUnknown[j1, j2] implies (j1 in used iff j2 in used)
}

// trace:formalizes RFC7517-KTY-REQUIRED
pred stdKtyRequired[j: JWK] { some j.kty }

// trace:formalizes RFC7517-KTY-FORMAT
pred stdKtyFormat[j: JWK] { j.kty in Text }

// trace:formalizes RFC7517-USE-FORMAT
pred stdUseFormat[j: JWK] { j.use in Text }

// The choice the MAY permits.
// trace:formalizes RFC7517-USE-OTHER-VALUES
pred stdUseOtherValue[j: JWK] { some j.use and j.use in Text - Sig - Enc }

// required tells whether the application requires use; the choice is to omit it.
// trace:formalizes RFC7517-USE-OPTIONAL
pred stdUseOmitted[j: JWK, required: Bool] { required = False and no j.use }

fun keyOperations: set Text { Sign + Verify + Encrypt + Decrypt + WrapKey + UnwrapKey + DeriveOp }

// trace:formalizes RFC7517-KEY-OPS-FORMAT
pred stdKeyOpsFormat[j: JWK] { some j.keyOps implies (j.keyOps in Array and values[j.keyOps] in Text) }

// trace:formalizes RFC7517-KEY-OPS-OTHER-VALUES
pred stdKeyOpsOtherValue[j: JWK] { some values[j.keyOps] & Text - keyOperations }

// trace:formalizes RFC7517-KEY-OPS-DUPLICATE
pred stdKeyOpsDuplicate[j: JWK] { not repeated[j.keyOps] }

// trace:formalizes RFC7517-KEY-OPS-OPTIONAL
pred stdKeyOpsOmitted[j: JWK, required: Bool] { required = False and no j.keyOps }

// trace:formalizes RFC7517-KEY-OPS-UNRELATED
pred stdKeyOpsRelated[j: JWK] {
  let ops = values[j.keyOps] |
    lone ops or ops in Sign + Verify or ops in Encrypt + Decrypt or ops in WrapKey + UnwrapKey
}

// trace:formalizes RFC7517-USE-KEY-OPS-TOGETHER
pred stdUseKeyOpsApart[j: JWK] { no j.use or no j.keyOps }

// RFC 7517 does not define consistent. The model reads sig as the signature
// operations and enc as the other operations.
// trace:formalizes RFC7517-USE-KEY-OPS-CONSISTENT
pred stdUseKeyOpsConsistent[j: JWK] {
  (some j.use and some j.keyOps) implies {
    j.use = Sig implies values[j.keyOps] in Sign + Verify
    j.use = Enc implies no values[j.keyOps] & (Sign + Verify)
  }
}

// trace:formalizes RFC7517-ALG-FORMAT
pred stdJwkAlgFormat[j: JWK] { j.alg in Text }

// trace:formalizes RFC7517-ALG-OPTIONAL
pred stdJwkAlgOmitted[j: JWK] { no j.alg }

// trace:formalizes RFC7517-KID-DISTINCT
pred stdKidDistinct[s: KeySet] {
  all disj j1, j2: jwks[s] | (some j1.kid and some j2.kid) implies j1.kid != j2.kid
}

// trace:formalizes RFC7517-KID-FORMAT
pred stdJwkKidFormat[j: JWK] { j.kid in Text }

// trace:formalizes RFC7517-KID-OPTIONAL
pred stdJwkKidOmitted[j: JWK] { no j.kid }

// The kid of a JWS matches the kid of a JWK when the two strings are equal.
// trace:formalizes RFC7517-KID-HEADER-MATCH RFC7515-KID-JWK-MATCH
pred stdKidMatches[t: JWT, j: JWK] { some t.kid and stdStringEqual[t.kid, j.kid] }

// trace:formalizes RFC7517-X5U-OPTIONAL
pred stdX5uOmitted[j: JWK] { no j.x5u }

// trace:formalizes RFC7517-X5C-OPTIONAL
pred stdX5cOmitted[j: JWK] { no j.x5c }

// trace:formalizes RFC7517-X5T-OPTIONAL
pred stdX5tOmitted[j: JWK] { no j.x5t }

// trace:formalizes RFC7517-X5T-S256-OPTIONAL
pred stdX5tS256Omitted[j: JWK] { no j.x5tS256 }

// ---- Standard layer: RFC 7517 JWK Set ----

// trace:formalizes RFC7517-JWKS-KEYS-REQUIRED
pred stdJwksKeysRequired[s: KeySet] { s.keysArray = True and items[s] in JWK }

// trace:unformalized RFC7517-JWKS-WHITESPACE モデルはデコード後の値を扱い、JSON のテキストの空白を扱わない

// trace:formalizes RFC7517-JWKS-MEMBER-NAME-UNIQUE
pred stdJwksMemberNameUnique[s: KeySet] { s.duplicateName = False }

// trace:formalizes RFC7517-JWKS-MEMBER-NAME-DUPLICATE
pred stdJwksMemberNameDuplicate[s: KeySet, used: Bool, lastWins: Bool] {
  (s.duplicateName = True and used = True) implies lastWins = True
}

pred sameKeySetExceptUnknown[s1, s2: KeySet] {
  s1.json = s2.json and s1.duplicateName = s2.duplicateName
  s1.keysArray = s2.keysArray and s1.elements = s2.elements
}

// decide is the outcome of the implementation for each JWK Set and JWT.
// trace:formalizes RFC7517-JWKS-MEMBER-UNKNOWN-IGNORE
pred stdJwksMemberUnknownIgnore[decide: KeySet -> JWT -> Outcome] {
  all s1, s2: KeySet, t: JWT | sameKeySetExceptUnknown[s1, s2] implies t.(s1.decide) = t.(s2.decide)
}

// ignorable are the JWKs whose kty the implementation does not understand,
// that lack a required member, or whose values are out of the supported range;
// used are the JWKs the implementation uses.
// trace:formalizes RFC7517-JWKS-KEY-IGNORE
pred stdJwksKeyIgnore[s: KeySet, ignorable: set JWK, used: set JWK] { no jwks[s] & ignorable & used }

// The keys of s2 are a permutation of those of s1: each element occurs as often.
pred sameKeySetExceptOrder[s1, s2: KeySet] {
  s1.json = s2.json and s1.duplicateName = s2.duplicateName and s1.unknown = s2.unknown
  s1.keysArray = s2.keysArray
  all x: Element | #(s1.elements).x = #(s2.elements).x
}

// ordered tells whether the application assigns a meaning to the order of the
// keys, which the Definition leaves open; by default it does not.
// trace:formalizes RFC7517-JWKS-KEYS-ORDER
pred stdJwksKeysOrder[decide: KeySet -> JWT -> Outcome, ordered: Bool] {
  ordered = False implies
    all s1, s2: KeySet, t: JWT | sameKeySetExceptOrder[s1, s2] implies t.(s1.decide) = t.(s2.decide)
}

// A string is its sequence of code points, so the comparison of two strings is
// the identity of their atoms.
// trace:formalizes RFC7517-STRING-COMPARE RFC7515-STRING-COMPARE
pred stdStringEqual[a, b: Value] { one a and a in Text and a = b }

// trace:unformalized RFC7517-NON-PUBLIC-PROTECT モデルは JWK を受け取る主体と、アクセスの制御を表さない

// ---- Standard layer: RFC 7518 and RFC 9964 ----

// trace:formalizes RFC7518-BASE64URLUINT-FORMAT
pred stdBase64urlUInt[v: Value] { v in Text and some v.uint }

// trace:formalizes RFC7518-BASE64URLUINT-MINIMAL
pred stdBase64urlUIntMinimal[v: Value] { v in Text and v.minimal = True }

// trace:formalizes RFC7518-RS256-VALIDATE
pred stdRS256Validates[t: JWT, p: PublicKey] { validates[t, p, RS256] }

// a is the algorithm the key is used with.
// trace:formalizes RFC7518-RS-KEY-SIZE
pred stdRsKeySize[p: PublicKey, a: Text] { a in PkcsAlg implies (p in RsaKey and p.modulus.bits2048 = True) }

// trace:formalizes RFC7518-PS256-VALIDATE
pred stdPS256Validates[t: JWT, p: PublicKey] { validates[t, p, PS256] }

// trace:formalizes RFC7518-PS-KEY-SIZE
pred stdPsKeySize[p: PublicKey, a: Text] { a in PssAlg implies (p in RsaKey and p.modulus.bits2048 = True) }

// trace:formalizes RFC7518-NONE-SIGNATURE-EMPTY
pred stdNoneSignatureEmpty[t: JWT] { t.alg = None implies t.emptySignature = True }

// trace:formalizes RFC7518-NONE-RECIPIENT-VERIFY
pred stdNoneRecipientVerify[v: Verification] {
  (v.outcome = Accepted and v.jwt.alg = None) implies v.jwt.emptySignature = True
}

// trace:formalizes RFC7518-EC-PRIVATE-D
pred stdEcPrivateKey[j: JWK] { EC in j.kty and D in j.secret }

// trace:formalizes RFC7518-RSA-KTY
pred stdRsaKey[j: JWK] { RSA in j.kty }

// trace:formalizes RFC7518-RSA-PUBLIC-MEMBERS
pred stdRsaPublicMembers[j: JWK] { stdRsaKey[j] implies (some j.n and some j.e) }

// trace:formalizes RFC7518-RSA-N-FORMAT
pred stdRsaNFormat[j: JWK] { (stdRsaKey[j] and some j.n) implies stdBase64urlUInt[j.n] }

// trace:formalizes RFC7518-RSA-E-FORMAT
pred stdRsaEFormat[j: JWK] { (stdRsaKey[j] and some j.e) implies stdBase64urlUInt[j.e] }

// trace:formalizes RFC7518-RSA-PRIVATE-MEMBERS
pred stdRsaPrivateKey[j: JWK] { stdRsaKey[j] and some j.secret & (D + P + Q + DP + DQ + QI + Oth) }

// trace:formalizes RFC7518-OCT-KEY-VALUE
pred stdSymmetricKey[j: JWK] { Oct in j.kty and K in j.secret }

// trace:formalizes RFC9964-AKP-ALG-REQUIRED
pred stdAkpAlgRequired[j: JWK] { AKP in j.kty implies some j.alg }

// trace:formalizes RFC9964-AKP-PUB-REQUIRED
pred stdAkpPubRequired[j: JWK] { AKP in j.kty implies some j.pub }

// publicKey is the part of the Condition that j represents a public key.
// trace:formalizes RFC9964-AKP-PRIV-PUBLIC-KEY
pred stdAkpPrivAbsent[j: JWK, publicKey: Bool] { (AKP in j.kty and publicKey = True) implies Priv not in j.secret }

// ---- Standard layer: OpenID Connect Discovery JWK Set ----

// Whether a key is for signatures or for encryption, as far as its members
// tell: use, key_ops, and for signatures the alg of a signature algorithm. The
// model has no alg of an encryption algorithm, so an encryption key that has
// neither use nor key_ops is not told apart.
pred signingKey[j: JWK] {
  j.use = Sig or some values[j.keyOps] & (Sign + Verify) or j.alg in PkcsAlg + PssAlg + EcdsaAlg + MacAlg
}
pred encryptionKey[j: JWK] { j.use = Enc or some values[j.keyOps] & (Encrypt + Decrypt + WrapKey + UnwrapKey) }

// t is a JWT the OpenID Provider signed.
// trace:formalizes OIDCD-JWKS-SIGNING-KEYS
pred stdJwksSigningKeys[s: KeySet, t: JWT] { some t.signer and some j: jwks[s] | t.signer = keyOf[j] }

// trace:formalizes OIDCD-JWKS-ENCRYPTION-KEYS
pred stdJwksEncryptionKeys[s: KeySet] { some j: jwks[s] | encryptionKey[j] }

// trace:formalizes OIDCD-JWKS-USE-REQUIRED
pred stdJwksUseRequired[s: KeySet] {
  ((some j: jwks[s] | signingKey[j]) and (some j: jwks[s] | encryptionKey[j])) implies all j: jwks[s] | some j.use
}

// trace:formalizes OIDCD-JWKS-KEY-DUAL-USE
pred stdJwksNoDualUse[s: KeySet] { no j: jwks[s] | signingKey[j] and encryptionKey[j] }

// trace:formalizes OIDCD-JWKS-X5C
pred stdJwksX5c[s: KeySet] { some j: jwks[s] | some j.x5c }

// The bare key values of an RSA key are n and e. The model has the bare key
// values of RSA keys only, so the clause constrains the RSA keys.
// trace:formalizes OIDCD-JWKS-X5C-BARE-KEY-PRESENT
pred stdX5cBareKeyPresent[j: JWK] { (RSA in j.kty and some j.x5c) implies (some j.n and some j.e) }

// trace:formalizes OIDCD-JWKS-X5C-BARE-KEY-MATCH
pred stdX5cBareKeyMatch[j: JWK] { (RSA in j.kty and some j.x5c) implies (some keyOf[j] and leaf[j].key = keyOf[j]) }

// The private and symmetric key values are the members that RFC7518-EC-PRIVATE-D,
// RFC7518-RSA-PRIVATE-MEMBERS, RFC7518-OCT-KEY-VALUE, and RFC9964-AKP-PRIV-PUBLIC-KEY
// name, whatever the kty of the JWK that holds them.
// trace:formalizes OIDCD-JWKS-NO-PRIVATE
pred stdJwksNoPrivate[s: KeySet] { no jwks[s].secret }

// ---- Standard layer: RFC 7515 JWS ----

// A key for use with an algorithm: an RSA key for the RSA algorithms, and no key
// for none. The model has no key of the other algorithms, so it does not tell
// whether another key is for use with them.
pred keyForAlg[v: Verification] {
  v.jwt.alg = None or (some v.key and (v.jwt.alg in PkcsAlg + PssAlg implies some keyOf[v.key]))
}

// supported is the set of algorithms the Recipient supports.
// trace:formalizes RFC7515-ALG-SIGNATURE-INVALID
pred stdAlgSignatureInvalid[v: Verification, supported: set Text] {
  (v.jwt.alg not in supported or not keyForAlg[v]) implies v.outcome != Accepted
}

// trace:formalizes RFC7515-ALG-ACCURATE
pred stdAlgAccurate[t: JWT] { some t.alg and t.alg = (some t.signingAlg => t.signingAlg else None) }

// trace:formalizes RFC7515-SIGNATURE-VALIDATE-ALG
pred stdSignatureValidateAlg[v: Verification] {
  v.outcome = Accepted implies (some v.jwt.alg and v.opAlg = v.jwt.alg)
}

// trace:formalizes RFC7515-SIGNATURE-VALID-REQUIRED
pred stdSignatureValidRequired[v: Verification] {
  v.outcome = Accepted implies validates[v.jwt, keyOf[v.key], v.opAlg]
}

// embedded is the Condition: the library uses the algorithm encoded in the signature value.
// trace:formalizes RFC7515-ALG-SIGNATURE-MATCH
pred stdAlgSignatureMatch[v: Verification, embedded: Bool] {
  (embedded = True and v.outcome = Accepted) implies v.jwt.signingAlg = v.jwt.alg
}

// trace:formalizes RFC7515-KEY-UNDETERMINED-FAIL
pred stdKeyUndeterminedFail[v: Verification] { (v.jwt.alg != None and no v.key) implies v.outcome != Accepted }

// ---- Standard layer: RFC 8725 ----

// configured is the algorithm the acceptance of the application is written in
// terms of, and current the cryptographically current algorithms. The design
// enables replacing the algorithm when every cryptographic operation uses the
// configured algorithm, so that replacing it replaces the algorithm of every
// operation, and the configured algorithm is current.
// trace:formalizes RFC8725-ALG-AGILITY
pred stdAlgAgility[vs: set Verification, configured: Text, current: set Text] {
  configured in current
  all v: vs | some v.opAlg implies v.opAlg = configured
}

// trace:formalizes RFC8725-ALG-HEADER-MATCH
pred stdAlgHeaderMatch[v: Verification] { some v.opAlg implies v.opAlg = v.jwt.alg }

// vs are the verifications of one application.
// trace:formalizes RFC8725-KEY-ALG-SINGLE
pred stdKeyAlgSingle[vs: set Verification] {
  all v1, v2: vs | (some keyOf[v1.key] and keyOf[v1.key] = keyOf[v2.key]) implies v1.opAlg = v2.opAlg
}

// keyAlg is the one algorithm the application binds each key to.
// trace:formalizes RFC8725-KEY-ALG-CHECK
pred stdKeyAlgCheck[v: Verification, keyAlg: JWK -> Value] { some v.opAlg implies v.opAlg = v.key.keyAlg }

// trace:formalizes RFC8725-CRYPTO-VALIDATE
pred stdCryptoValidate[v: Verification] {
  v.outcome = Accepted implies validates[v.jwt, keyOf[v.key], v.opAlg]
}

// innerValid stands for the inner operations validating with the keys and
// algorithms the application supplies; the model does not represent the inner JWT.
// trace:formalizes RFC8725-CRYPTO-NESTED-VALIDATE
pred stdCryptoNestedValidate[v: Verification] {
  (v.outcome = Accepted and v.jwt.nested = True) implies v.jwt.innerValid = True
}

// ---- Standard layer: OpenID Connect Core ----

// trace:formalizes OIDCC-ID-TOKEN-SIGNED
pred stdIdTokenSigned[t: JWT, idToken: Bool] { idToken = True implies some t.signer }

// trace:unformalized OIDCC-CODE-TLS-ISSUER モデルは TLS のサーバーの検証を扱わない

// idToken and direct are the Condition: v.jwt is an ID Token, and direct tells
// whether it is received directly from the Token Endpoint.
// trace:formalizes OIDCC-CODE-SIGNATURE-VALIDATE
pred stdCodeSignatureValidate[v: Verification, idToken: Bool, direct: Bool] {
  (idToken = True and direct = False and v.outcome = Accepted) implies (v.opAlg = v.jwt.alg and validates[v.jwt, keyOf[v.key], v.opAlg])
}

// issuerKeys are the keys the Issuer provides.
// trace:formalizes OIDCC-CODE-ISSUER-KEYS
pred stdCodeIssuerKeys[v: Verification, codeFlow: Bool, issuerKeys: set JWK] {
  (codeFlow = True and v.outcome = Accepted) implies (some v.key and v.key in issuerKeys)
}

// trace:formalizes OIDCC-IMPLICIT-SIGNATURE
pred stdImplicitSignature[v: Verification, implicit: Bool] {
  (implicit = True and v.outcome = Accepted) implies (v.opAlg = v.jwt.alg and validates[v.jwt, keyOf[v.key], v.opAlg])
}

// trace:formalizes OIDCC-SIGNING-ALG-SELECT
pred stdSigningAlgSelect[t: JWT, supported: set Text] { t.alg in supported }

// family is the kty of the signing key.
// trace:formalizes OIDCC-SIGNING-ASYMMETRIC-ALG
pred stdSigningAsymmetricAlg[t: JWT, family: Text] {
  family = RSA implies t.alg in PkcsAlg + PssAlg
  family = EC implies t.alg in EcdsaAlg
}

// asymmetric is the Condition: the signature is an RSA or ECDSA signature.
// trace:formalizes OIDCC-SIGNING-KEY-PUBLISHED
pred stdSigningKeyPublished[t: JWT, s: KeySet, asymmetric: Bool] {
  asymmetric = True implies (some t.signer and some j: jwks[s] | t.signer = keyOf[j])
}

// trace:formalizes OIDCC-SIGNING-KID
pred stdSigningKid[t: JWT, s: KeySet, asymmetric: Bool] { (asymmetric = True and #jwks[s] > 1) implies some t.kid }

// trace:formalizes OIDCC-SIGNING-KEY-USAGE
pred stdSigningKeyUsage[t: JWT, s: KeySet, asymmetric: Bool] {
  all j: jwks[s] | (asymmetric = True and some t.signer and keyOf[j] = t.signer) implies {
    no j.use or j.use = Sig
    no j.keyOps or some values[j.keyOps] & (Sign + Verify)
  }
}

// trace:formalizes OIDCC-SIGNING-MAC-ALG
pred stdSigningMacAlg[t: JWT, mac: Bool] { mac = True implies t.alg in MacAlg }

// trace:formalizes OIDCC-SIGNING-MAC-PUBLIC-CLIENT
pred stdSigningMacPublicClient[v: Verification, publicClient: Bool] {
  (publicClient = True and v.outcome = Accepted) implies v.jwt.alg not in MacAlg
}

// trace:unformalized OIDCC-KEY-ROTATION-RETAIN モデルは時間の経過と JWK Set の入れ替えを表さない

// ---- GitHub layer: Published and Observed ----

// trace:formalizes GH-OIDC-JWKS-RSA-MEMBERS
pred ghJwksRsaMembers[s: KeySet] {
  s.json = True and s.duplicateName = False and no s.unknown and s.keysArray = True
  #s.elements = 4 and items[s] in JWK
  not (some disj i, k: (s.elements).Element | (s.elements)[i] = (s.elements)[k])
  all j: jwks[s] {
    j.duplicateName = False
    j.kty = RSA and j.alg = RS256 and j.use = Sig and j.e = AQAB
    some j.kid and some j.n
    no j.keyOps and no j.pub and no j.x5u and no j.x5tS256 and no j.secret and no j.unknown
  }
}

// trace:formalizes GH-OIDC-JWKS-MODULUS
pred ghJwksModulus[s: KeySet] {
  all j: jwks[s] | j.n.minimal = True and j.n.uint.bits2048 = True and j.n.uint.odd = True
}

// trace:formalizes GH-OIDC-JWKS-X5C
pred ghJwksX5c[s: KeySet] {
  #{ j: jwks[s] | some j.x5c } = 3
  all j: jwks[s] | some j.x5c iff some j.x5t
  all j: jwks[s] | some j.x5c implies (j.x5c in Array and #j.x5c.elements = 1 and some leaf[j])
}

// trace:formalizes GH-OIDC-JWKS-X5C-KEY
pred ghJwksX5cKey[s: KeySet] {
  all j: jwks[s] | some j.x5c implies {
    leaf[j].key = keyOf[j] and some keyOf[j]
    j.x5t = leaf[j].sha1
    leaf[j].keyUsageExtension = False or leaf[j].keyUsage = DigitalSignature + KeyEncipherment
  }
  #{ j: jwks[s] | some j.x5c and leaf[j].keyUsageExtension = True } = 2
}

// The model does not tell the UUID and the hexadecimal forms apart.
// trace:formalizes GH-OIDC-JWKS-KID-FORMAT
pred ghJwksKidFormat[s: KeySet] { all j: jwks[s] | some j.kid and j.kid in Text - Empty }

// trace:formalizes GH-OIDC-JWKS-KID-DISTINCT
pred ghJwksKidDistinct[s: KeySet] { all disj j1, j2: jwks[s] | j1.kid != j2.kid }

// The model has no x5t of the JOSE Header.
// trace:formalizes GH-OIDC-HEADER-KID-JWKS
pred ghHeaderKidJwks[t: JWT, s: KeySet] { some t.kid and some j: jwks[s] | t.kid = j.kid }

// The Header consists of alg (RS256), kid, typ, and x5t only, so it has no cty
// and the payload is not a JWT. The model has no typ or x5t of the Header.
// trace:formalizes GH-OIDC-HEADER-OBSERVED
pred ghHeaderObserved[t: JWT] { t.alg = RS256 and some t.kid and t.nested = False }

pred ghPublished[s: KeySet] {
  ghJwksRsaMembers[s] and ghJwksModulus[s] and ghJwksX5c[s] and ghJwksX5cKey[s]
  ghJwksKidFormat[s] and ghJwksKidDistinct[s]
}

// ---- GATES layer ----

// The one algorithm GOJ-HEADER-ALG, GOJ-JWK-ALG, and GOJ-SIGNATURE accept. The
// model leaves it free among the RSASSA-PKCS1-v1_5 algorithms, so that the
// safety checks hold whichever of them GATES is written for; GATES accepts
// RS256 (gatesCurrent), and the compatibility checks assume it.
one sig GatesAlgorithm { value: one PkcsAlg }
fun gatesAlgorithm: Text { GatesAlgorithm.value }
pred gatesCurrent { gatesAlgorithm = RS256 }

// The JWT has passed GOJ-HEADER-ALG, GOJ-HEADER-KID, and GOJ-HEADER-CTY, which
// the Z3 model formalizes, before GATES selects a key.
pred gatesHeader[t: JWT] { t.alg = gatesAlgorithm and some t.kid and t.kid in Text - Empty and t.nested = False }

// trace:formalizes GOJ-JWK-JSON
pred gatesJwkJson[j: JWK] { j.duplicateName = False }

// trace:formalizes GOJ-JWK-RSA
pred gatesJwkRsa[j: JWK] {
  j.kty = RSA
  j.n.minimal = True and j.n.uint.bits2048 = True and j.n.uint.odd = True
  j.e.minimal = True and j.e.uint.atLeast3 = True and j.e.uint.odd = True and j.e.uint.atMost2p31 = True
}

// trace:formalizes GOJ-JWK-KID
pred gatesJwkKid[j: JWK] { some j.kid and j.kid in Text - Empty }

// trace:formalizes GOJ-JWK-USAGE
pred gatesJwkUsage[j: JWK] {
  some j.use implies j.use = Sig
  some j.keyOps implies {
    j.keyOps in Array and values[j.keyOps] in Text
    Verify in values[j.keyOps]
    not repeated[j.keyOps]
    some j.use implies values[j.keyOps] in Sign + Verify
  }
}

// trace:formalizes GOJ-JWK-ALG
pred gatesJwkAlg[j: JWK] { some j.alg implies j.alg = gatesAlgorithm }

// trace:formalizes GOJ-X5C-KEY
pred gatesX5cKey[j: JWK] {
  some j.x5c implies {
    j.x5c in Array and some j.x5c.elements
    all c: values[j.x5c] | c in Text and some c.certificate
    some keyOf[j] and leaf[j].key = keyOf[j]
    leaf[j].keyUsageExtension = True implies DigitalSignature in leaf[j].keyUsage
  }
  some j.x5t implies (j.x5t in Text and j.x5t.sha1Form = True and (some j.x5c implies j.x5t = leaf[j].sha1))
  some j.x5tS256 implies (j.x5tS256 in Text and j.x5tS256.sha256Form = True and (some j.x5c implies j.x5tS256 = leaf[j].sha256))
  some j.x5u implies (j.x5u in Text and j.x5u.url = True)
}

// trace:formalizes GOJ-X5C-CHAIN
pred gatesX5cChain[j: JWK] {
  all i: (j.x5c.elements).Value - 0 |
    (j.x5c.elements)[i].certificate.key in (j.x5c.elements)[minus[i, 1]].certificate.signedBy
}

// trace:formalizes GOJ-JWKS-USABLE
pred gatesUsable[j: JWK] {
  gatesJwkJson[j] and gatesJwkRsa[j] and gatesJwkKid[j] and gatesJwkUsage[j] and gatesJwkAlg[j]
  gatesX5cKey[j] and gatesX5cChain[j]
}

// trace:formalizes GOJ-JWKS-SECRET
pred gatesSecret[j: JWK] { Oct in j.kty or some j.secret }

// trace:formalizes GOJ-JWKS-FORMAT
pred gatesJwksFormat[s: KeySet] { s.json = True and s.duplicateName = False and s.keysArray = True }

fun usableKeys[s: KeySet]: set JWK { { j: jwks[s] | gatesUsable[j] } }

// GATES keeps the JWK Set only when it has the form, holds no secret value,
// and has a usable key.
pred gatesKeySet[s: KeySet] {
  gatesJwksFormat[s]
  no j: jwks[s] | gatesSecret[j]
  some usableKeys[s]
}

fun candidates[s: KeySet, t: JWT]: set JWK { { j: usableKeys[s] | stdStringEqual[t.kid, j.kid] } }

// trace:formalizes GOJ-JWKS-KID
fun gatesKey[s: KeySet, t: JWT]: lone JWK {
  (gatesHeader[t] and gatesKeySet[s] and one candidates[s, t]) => candidates[s, t] else none
}

// trace:formalizes GOJ-SIGNATURE
pred gatesSignature[t: JWT, j: JWK] { validates[t, keyOf[j], gatesAlgorithm] }

fun gatesOutcome[s: KeySet, t: JWT]: one Outcome {
  not gatesHeader[t] => AuthenticationFailure
  else not gatesKeySet[s] => InternalFailure
  else (some gatesKey[s, t] and gatesSignature[t, gatesKey[s, t]]) => Accepted
  else AuthenticationFailure
}

fun gatesDecision: KeySet -> JWT -> Outcome { { s: KeySet, t: JWT, o: Outcome | o = gatesOutcome[s, t] } }

pred gatesVerification[v: Verification] {
  v.outcome = gatesOutcome[v.keySet, v.jwt]
  v.key = gatesKey[v.keySet, v.jwt]
  v.opAlg = (some v.key => gatesAlgorithm else none)
}

// The algorithm GATES binds a key to: the alg of the JWK, and the algorithm of
// GOJ-HEADER-ALG when the JWK has none.
fun gatesKeyAlg: JWK -> Value { { j: JWK, a: Value | a = (some j.alg => j.alg else gatesAlgorithm) } }

// ---- Mutations of the GATES layer ----

// Uses one of the usable keys of the kid even when several have it, as a
// library that takes the first key of a kid does.
pred mutantSelectsAnyCandidate[s: KeySet, t: JWT, j: JWK] {
  gatesHeader[t] and gatesKeySet[s] and j in candidates[s, t]
}

// Excludes the JWKs that hold a secret value instead of rejecting the JWK Set.
pred mutantKeySetExcludingSecret[s: KeySet] {
  gatesJwksFormat[s]
  some usableKeys[s] - { j: JWK | gatesSecret[j] }
}

// Treats a JWK Set without a usable key as an authentication failure.
fun mutantOutcomeNoUsableKey[s: KeySet, t: JWT]: one Outcome {
  (gatesHeader[t] and gatesJwksFormat[s] and no j: jwks[s] | gatesSecret[j]) and no usableKeys[s] => AuthenticationFailure
  else gatesOutcome[s, t]
}

// Uses an RSA key with a modulus shorter than 2048 bits.
pred mutantJwkRsaShortModulus[j: JWK] {
  j.kty = RSA
  j.n.minimal = True and j.n.uint.odd = True
  j.e.minimal = True and j.e.uint.atLeast3 = True and j.e.uint.odd = True and j.e.uint.atMost2p31 = True
}

// ---- Consistency ----

pred StandardLayerConsistent {
  some s: KeySet, t: JWT, v: Verification {
    v.keySet = s and v.jwt = t and v.outcome = Accepted
    stdJwksKeysRequired[s] and stdJwksMemberNameUnique[s] and stdJwksNoPrivate[s] and stdKidDistinct[s]
    stdJwksUseRequired[s] and stdJwksNoDualUse[s] and stdJwksX5c[s] and stdJwksSigningKeys[s, t]
    all j: jwks[s] {
      stdJwkMemberNameUnique[j] and stdKtyRequired[j] and stdKtyFormat[j] and stdUseFormat[j]
      stdKeyOpsFormat[j] and stdKeyOpsDuplicate[j] and stdKeyOpsRelated[j] and stdUseKeyOpsConsistent[j]
      stdJwkAlgFormat[j] and stdJwkKidFormat[j]
      stdRsaPublicMembers[j] and stdRsaNFormat[j] and stdRsaEFormat[j]
      stdBase64urlUIntMinimal[j.n] and stdBase64urlUIntMinimal[j.e]
      stdAkpAlgRequired[j] and stdAkpPubRequired[j] and stdAkpPrivAbsent[j, True]
      stdX5cBareKeyPresent[j] and stdX5cBareKeyMatch[j]
    }
    stdAlgAccurate[t] and stdNoneSignatureEmpty[t] and stdIdTokenSigned[t, True]
    stdSigningAlgSelect[t, RS256] and stdSigningAsymmetricAlg[t, RSA] and stdSigningKeyPublished[t, s, True]
    stdSigningKid[t, s, True] and stdSigningKeyUsage[t, s, True] and stdSigningMacAlg[t, False]
    stdRS256Validates[t, keyOf[v.key]] and stdRsKeySize[keyOf[v.key], RS256]
    stdSignatureValidateAlg[v] and stdSignatureValidRequired[v] and stdAlgSignatureMatch[v, True]
    stdAlgSignatureInvalid[v, RS256] and stdKeyUndeterminedFail[v] and stdNoneRecipientVerify[v]
    stdAlgHeaderMatch[v] and stdKeyAlgSingle[v] and stdKeyAlgCheck[v, JWK -> RS256]
    stdCryptoValidate[v] and stdCryptoNestedValidate[v]
    stdCodeSignatureValidate[v, True, False] and stdCodeIssuerKeys[v, True, jwks[s]] and stdImplicitSignature[v, True]
    stdSigningMacPublicClient[v, True]
  }
}

pred GitHubPublishedConsistent {
  some s: KeySet, t: JWT | ghPublished[s] and ghHeaderKidJwks[t, s] and ghHeaderObserved[t]
}

pred GatesLayerConsistent {
  some v: Verification | gatesVerification[v] and v.outcome = Accepted and some v.key.x5c and #v.key.x5c.elements = 2
}

pred GatesWithAnotherAlgorithm {
  not gatesCurrent and some v: Verification | gatesVerification[v] and v.outcome = Accepted
}

pred GatesLayerFailures {
  some v1, v2: Verification | gatesVerification[v1] and gatesVerification[v2] and gatesHeader[v1.jwt] and gatesHeader[v2.jwt]
    and v1.outcome = InternalFailure and v2.outcome = AuthenticationFailure
}

run StandardLayerConsistent for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 4 UInt, 2 Certificate, 1 Name, 4 seq expect 1
// trace:checks GH-OIDC-JWKS-RSA-MEMBERS GH-OIDC-JWKS-MODULUS GH-OIDC-JWKS-X5C GH-OIDC-JWKS-X5C-KEY GH-OIDC-JWKS-KID-FORMAT GH-OIDC-JWKS-KID-DISTINCT GH-OIDC-HEADER-KID-JWKS GH-OIDC-HEADER-OBSERVED
run GitHubPublishedConsistent for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 1
// trace:checks GOJ-JWK-JSON GOJ-JWK-RSA GOJ-JWK-KID GOJ-JWK-USAGE GOJ-JWK-ALG GOJ-X5C-KEY GOJ-X5C-CHAIN GOJ-JWKS-FORMAT GOJ-JWKS-SECRET GOJ-JWKS-USABLE GOJ-JWKS-KID GOJ-SIGNATURE
run GatesLayerConsistent for 24 but 1 KeySet, 1 JWT, 1 Verification, 2 JWK, 2 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 1
// trace:checks GOJ-JWKS-FORMAT GOJ-JWKS-SECRET GOJ-JWKS-USABLE GOJ-JWKS-KID
run GatesLayerFailures for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 1
// GATES written for an algorithm other than RS256 still accepts a JWT, so the
// safety checks cover such algorithms and do not hold only for RS256.
// trace:checks GOJ-HEADER-ALG GOJ-JWK-ALG GOJ-SIGNATURE
run GatesWithAnotherAlgorithm for 24 but 1 KeySet, 1 JWT, 1 Verification, 2 JWK, 2 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 1

// ---- Safety: A satisfies S ----

assert JwkWithoutDuplicateName {
  all j: JWK | gatesJwkJson[j] implies (stdJwkMemberNameUnique[j] and stdJwkMemberNameDuplicate[j, True, False])
}

assert UnknownJwkMemberIgnored { stdJwkMemberUnknownIgnore[{ j: JWK | gatesUsable[j] }] }

assert RsaJwkIsWellFormed {
  all j: JWK | gatesJwkRsa[j] implies {
    stdRsaKey[j] and stdRsaPublicMembers[j] and stdRsaNFormat[j] and stdRsaEFormat[j]
    stdBase64urlUInt[j.n] and stdBase64urlUInt[j.e] and stdBase64urlUIntMinimal[j.n] and stdBase64urlUIntMinimal[j.e]
  }
}

assert RsaJwkHasLongModulus { all j: JWK | gatesJwkRsa[j] implies stdRsKeySize[keyOf[j], gatesAlgorithm] }

assert JwkKidIsString { all j: JWK | gatesJwkKid[j] implies stdJwkKidFormat[j] }

assert JwkUsageIsConsistent {
  all j: JWK | gatesJwkUsage[j] implies (stdKeyOpsDuplicate[j] and stdUseKeyOpsConsistent[j])
}

assert KeyUsedWithOneAlgorithm { stdKeyAlgSingle[{ v: Verification | gatesVerification[v] }] }

assert CertificateMatchesBareKey {
  all j: JWK | (gatesJwkRsa[j] and gatesX5cKey[j]) implies (stdX5cBareKeyPresent[j] and stdX5cBareKeyMatch[j])
}

// GATES requires keys to be an array and uses only its elements that are JWKs.
assert KeysIsArrayOfUsedJwks {
  all s: KeySet | gatesKeySet[s] implies (s.keysArray = True and usableKeys[s] in jwks[s])
}

assert KeySetWithoutDuplicateName {
  all s: KeySet | gatesJwksFormat[s] implies (stdJwksMemberNameUnique[s] and stdJwksMemberNameDuplicate[s, True, False])
}

assert UnknownKeySetMemberIgnored { stdJwksMemberUnknownIgnore[gatesDecision] }

assert KeptKeySetHasNoSecret {
  all s: KeySet | gatesKeySet[s] implies (stdJwksNoPrivate[s] and all j: jwks[s] | stdAkpPrivAbsent[j, True])
}

// The JWKs GATES excludes, other than those holding a secret value, are the
// ones GATES does not understand or support.
assert ExcludedJwkIsNeverUsed {
  all s: KeySet, t: JWT |
    stdJwksKeyIgnore[s, { j: jwks[s] | not gatesUsable[j] and not gatesSecret[j] }, gatesKey[s, t]]
}

assert SelectedKeyMatchesKid {
  all s: KeySet, t: JWT | some gatesKey[s, t] implies stdKidMatches[t, gatesKey[s, t]]
}

assert AcceptedSignatureValidates {
  all v: Verification | gatesVerification[v] implies {
    stdSignatureValidRequired[v] and stdSignatureValidateAlg[v] and stdCryptoValidate[v]
    (v.outcome = Accepted and gatesCurrent) implies stdRS256Validates[v.jwt, keyOf[v.key]]
  }
}

// Whichever RSASSA-PKCS1-v1_5 algorithm GATES is written for, every operation
// uses it, and the accepted JWTs keep the clauses on the algorithm and the key.
assert AlgorithmReplaceable {
  stdAlgAgility[{ v: Verification | gatesVerification[v] }, gatesAlgorithm, PkcsAlg]
  all v: Verification | gatesVerification[v] implies {
    stdSignatureValidRequired[v] and stdSignatureValidateAlg[v] and stdAlgHeaderMatch[v]
    stdKeyAlgCheck[v, gatesKeyAlg]
    some v.key implies stdRsKeySize[keyOf[v.key], v.opAlg]
  }
}

assert OperationUsesHeaderAndKeyAlgorithm {
  all v: Verification | gatesVerification[v] implies (stdAlgHeaderMatch[v] and stdKeyAlgCheck[v, gatesKeyAlg])
}

// The order of keys does not change the outcome.
assert KeyOrderIgnored { stdJwksKeysOrder[gatesDecision, False] }

// trace:checks GOJ-JWK-JSON RFC7517-MEMBER-NAME-UNIQUE RFC7517-MEMBER-NAME-DUPLICATE
check JwkWithoutDuplicateName for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 2 Name, 4 seq expect 0
// trace:checks GOJ-JWK-JSON RFC7517-MEMBER-UNKNOWN-IGNORE
check UnknownJwkMemberIgnored for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 2 Name, 4 seq expect 0
// trace:checks GOJ-JWK-RSA RFC7518-RSA-KTY RFC7518-RSA-PUBLIC-MEMBERS RFC7518-RSA-N-FORMAT RFC7518-RSA-E-FORMAT RFC7518-BASE64URLUINT-FORMAT RFC7518-BASE64URLUINT-MINIMAL
check RsaJwkIsWellFormed for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWK-RSA RFC7518-RS-KEY-SIZE
check RsaJwkHasLongModulus for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWK-KID RFC7517-KID-FORMAT
check JwkKidIsString for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWK-USAGE RFC7517-KEY-OPS-DUPLICATE RFC7517-USE-KEY-OPS-CONSISTENT
check JwkUsageIsConsistent for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWK-ALG RFC8725-KEY-ALG-SINGLE
check KeyUsedWithOneAlgorithm for 24 but 2 KeySet, 2 JWT, 3 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-X5C-KEY OIDCD-JWKS-X5C-BARE-KEY-PRESENT OIDCD-JWKS-X5C-BARE-KEY-MATCH
check CertificateMatchesBareKey for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-FORMAT RFC7517-JWKS-KEYS-REQUIRED
check KeysIsArrayOfUsedJwks for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-FORMAT RFC7517-JWKS-MEMBER-NAME-UNIQUE RFC7517-JWKS-MEMBER-NAME-DUPLICATE
check KeySetWithoutDuplicateName for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-FORMAT RFC7517-JWKS-MEMBER-UNKNOWN-IGNORE
check UnknownKeySetMemberIgnored for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 2 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-SECRET OIDCD-JWKS-NO-PRIVATE RFC9964-AKP-PRIV-PUBLIC-KEY
check KeptKeySetHasNoSecret for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-USABLE RFC7517-JWKS-KEY-IGNORE
check ExcludedJwkIsNeverUsed for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-KID RFC7515-KID-JWK-MATCH RFC7517-KID-HEADER-MATCH RFC7515-STRING-COMPARE RFC7517-STRING-COMPARE
check SelectedKeyMatchesKid for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-SIGNATURE RFC7515-SIGNATURE-VALID-REQUIRED RFC7515-SIGNATURE-VALIDATE-ALG RFC7518-RS256-VALIDATE RFC8725-CRYPTO-VALIDATE
check AcceptedSignatureValidates for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-SIGNATURE RFC8725-ALG-HEADER-MATCH RFC8725-KEY-ALG-CHECK
check OperationUsesHeaderAndKeyAlgorithm for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-HEADER-ALG RFC8725-ALG-AGILITY
check AlgorithmReplaceable for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks RFC7517-JWKS-KEYS-ORDER
check KeyOrderIgnored for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0

// ---- Properties of the GATES layer ----

// The key GATES uses is a usable key of the JWK Set it retrieved.
assert SelectedKeyIsUsableKeyOfKeySet { all s: KeySet, t: JWT | gatesKey[s, t] in usableKeys[s] & jwks[s] }

// A kid that matches several usable keys is an authentication failure.
assert AmbiguousKidFails {
  all s: KeySet, t: JWT | (gatesHeader[t] and gatesKeySet[s] and #candidates[s, t] > 1) implies gatesOutcome[s, t] = AuthenticationFailure
}

// A secret value anywhere rejects the whole JWK Set.
assert SecretRejectsKeySet {
  all s: KeySet, t: JWT | (gatesHeader[t] and some j: jwks[s] | gatesSecret[j]) implies gatesOutcome[s, t] = InternalFailure
}

// No usable key is an internal failure, not an authentication failure.
assert NoUsableKeyIsInternalFailure {
  all s: KeySet, t: JWT | (gatesHeader[t] and (not gatesJwksFormat[s] or no usableKeys[s])) implies gatesOutcome[s, t] = InternalFailure
}

// trace:checks GOJ-JWKS-USABLE GOJ-JWKS-KID
check SelectedKeyIsUsableKeyOfKeySet for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-KID
check AmbiguousKidFails for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-SECRET
check SecretRejectsKeySet for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-USABLE GOJ-JWKS-FORMAT
check NoUsableKeyIsInternalFailure for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 0

// ---- Mutations: the checks find a wrong GATES layer ----

assert MutantFirstMatchSelectsOneCandidate {
  all s: KeySet, t: JWT, j: JWK | mutantSelectsAnyCandidate[s, t, j] implies one candidates[s, t]
}

assert MutantExcludingSecretKeepsNoPrivate {
  all s: KeySet | mutantKeySetExcludingSecret[s] implies stdJwksNoPrivate[s]
}

assert MutantNoUsableKeyIsInternalFailure {
  all s: KeySet, t: JWT | (gatesHeader[t] and gatesJwksFormat[s] and no usableKeys[s]) implies mutantOutcomeNoUsableKey[s, t] = InternalFailure
}

assert MutantShortModulusHasLongModulus {
  all j: JWK | mutantJwkRsaShortModulus[j] implies stdRsKeySize[keyOf[j], RS256]
}

check MutantFirstMatchSelectsOneCandidate for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 1
check MutantExcludingSecretKeepsNoPrivate for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 1
check MutantNoUsableKeyIsInternalFailure for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 1
check MutantShortModulusHasLongModulus for 24 but 2 KeySet, 2 JWT, 2 Verification, 3 JWK, 3 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 1

// ---- Compatibility: G is accepted by A ----

assert GitHubJwkJsonAccepted { all s: KeySet | (gatesCurrent and ghPublished[s]) implies all j: jwks[s] | gatesJwkJson[j] }
assert GitHubJwkRsaAccepted { all s: KeySet | (gatesCurrent and ghPublished[s]) implies all j: jwks[s] | gatesJwkRsa[j] }
assert GitHubJwkKidAccepted { all s: KeySet | (gatesCurrent and ghPublished[s]) implies all j: jwks[s] | gatesJwkKid[j] }
assert GitHubJwkUsageAccepted { all s: KeySet | (gatesCurrent and ghPublished[s]) implies all j: jwks[s] | gatesJwkUsage[j] }
assert GitHubJwkAlgAccepted { all s: KeySet | (gatesCurrent and ghPublished[s]) implies all j: jwks[s] | gatesJwkAlg[j] }
assert GitHubX5cKeyAccepted { all s: KeySet | (gatesCurrent and ghPublished[s]) implies all j: jwks[s] | gatesX5cKey[j] }
// A single certificate has no relation to check.
assert GitHubX5cChainAccepted { all s: KeySet | (gatesCurrent and ghPublished[s]) implies all j: jwks[s] | gatesX5cChain[j] }
assert GitHubJwksFormatAccepted { all s: KeySet | (gatesCurrent and ghPublished[s]) implies gatesJwksFormat[s] }
assert GitHubJwksSecretAccepted { all s: KeySet | (gatesCurrent and ghPublished[s]) implies no j: jwks[s] | gatesSecret[j] }
// ghJwksRsaMembers gives four JWKs, so a usable key exists.
assert GitHubJwksUsableAccepted { all s: KeySet | (gatesCurrent and ghPublished[s]) implies all j: jwks[s] | gatesUsable[j] }
assert GitHubKidSelectsOneKey {
  all s: KeySet, t: JWT | (gatesCurrent and ghPublished[s] and ghHeaderKidJwks[t, s] and ghHeaderObserved[t]) implies one gatesKey[s, t]
}

// A JWK whose second certificate did not sign the first is excluded, and a
// JWT signed with its key fails when no other key has its kid. No Documented
// clause constrains the JWK Set, and GH-OIDC-JWKS-X5C-CHAIN leaves the
// certificates of x5c unsettled.
pred GitHubX5cChainMayBeRejected {
  some s: KeySet, t: JWT, j: jwks[s] {
    gatesJwkJson[j] and gatesJwkRsa[j] and gatesJwkKid[j] and gatesJwkUsage[j] and gatesJwkAlg[j] and gatesX5cKey[j]
    not gatesX5cChain[j]
    gatesHeader[t] and t.kid = j.kid and gatesSignature[t, j]
    gatesKeySet[s]
    gatesOutcome[s, t] = AuthenticationFailure
  }
}

// trace:checks GOJ-JWK-JSON GH-OIDC-JWKS-RSA-MEMBERS
check GitHubJwkJsonAccepted for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWK-RSA GH-OIDC-JWKS-RSA-MEMBERS GH-OIDC-JWKS-MODULUS
check GitHubJwkRsaAccepted for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWK-KID GH-OIDC-JWKS-RSA-MEMBERS GH-OIDC-JWKS-KID-FORMAT
check GitHubJwkKidAccepted for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWK-USAGE GH-OIDC-JWKS-RSA-MEMBERS
check GitHubJwkUsageAccepted for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWK-ALG GH-OIDC-JWKS-RSA-MEMBERS
check GitHubJwkAlgAccepted for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-X5C-KEY GH-OIDC-JWKS-X5C GH-OIDC-JWKS-X5C-KEY
check GitHubX5cKeyAccepted for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-X5C-CHAIN GH-OIDC-JWKS-X5C
check GitHubX5cChainAccepted for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-FORMAT GH-OIDC-JWKS-RSA-MEMBERS
check GitHubJwksFormatAccepted for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-SECRET GH-OIDC-JWKS-RSA-MEMBERS
check GitHubJwksSecretAccepted for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-USABLE GH-OIDC-JWKS-RSA-MEMBERS GH-OIDC-JWKS-MODULUS GH-OIDC-JWKS-X5C GH-OIDC-JWKS-X5C-KEY GH-OIDC-JWKS-KID-FORMAT
check GitHubJwksUsableAccepted for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-JWKS-KID GH-OIDC-HEADER-KID-JWKS GH-OIDC-HEADER-OBSERVED GH-OIDC-JWKS-KID-DISTINCT
check GitHubKidSelectsOneKey for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
// trace:checks GOJ-X5C-CHAIN GH-OIDC-JWKS-X5C-CHAIN
run GitHubX5cChainMayBeRejected for 24 but 1 KeySet, 1 JWT, 1 Verification, 2 JWK, 2 Element, 3 PublicKey, 3 UInt, 2 Certificate, 1 Name, 4 seq expect 1

// ---- Compatibility: the GATES layer depends on the Published clauses ----

// No Documented clause constrains the JWK Set, so each of these JWK
// conditions holds for GitHub only through the Published clauses.
pred JwkJsonNeedsPublished { some j: JWK | not gatesJwkJson[j] }
pred JwkRsaNeedsPublished { some j: JWK | gatesJwkJson[j] and not gatesJwkRsa[j] }
pred JwkKidNeedsPublished { some j: JWK | gatesJwkRsa[j] and not gatesJwkKid[j] }
pred JwkUsageNeedsPublished { some j: JWK | gatesJwkRsa[j] and not gatesJwkUsage[j] }
pred JwkAlgNeedsPublished { some j: JWK | gatesJwkRsa[j] and not gatesJwkAlg[j] }
pred X5cKeyNeedsPublished { some j: JWK | gatesJwkRsa[j] and not gatesX5cKey[j] }
pred JwksFormatNeedsPublished { some s: KeySet | not gatesJwksFormat[s] }
pred JwksSecretNeedsPublished { some s: KeySet | gatesJwksFormat[s] and some j: jwks[s] | gatesSecret[j] }
pred JwksUsableNeedsPublished { some s: KeySet | gatesJwksFormat[s] and some jwks[s] and no usableKeys[s] }
pred JwksKidNeedsPublished { some s: KeySet, t: JWT | gatesHeader[t] and gatesKeySet[s] and #candidates[s, t] > 1 }

// trace:checks GOJ-JWK-JSON
run JwkJsonNeedsPublished for 24 but 1 KeySet, 1 JWT, 1 Verification, 1 JWK, 1 Element, 1 PublicKey, 1 UInt, 1 Certificate, 1 Name, 4 seq expect 1
// trace:checks GOJ-JWK-RSA
run JwkRsaNeedsPublished for 24 but 1 KeySet, 1 JWT, 1 Verification, 1 JWK, 1 Element, 1 PublicKey, 1 UInt, 1 Certificate, 1 Name, 4 seq expect 1
// trace:checks GOJ-JWK-KID
run JwkKidNeedsPublished for 24 but 1 KeySet, 1 JWT, 1 Verification, 1 JWK, 1 Element, 2 PublicKey, 2 UInt, 1 Certificate, 1 Name, 4 seq expect 1
// trace:checks GOJ-JWK-USAGE
run JwkUsageNeedsPublished for 24 but 1 KeySet, 1 JWT, 1 Verification, 1 JWK, 1 Element, 2 PublicKey, 2 UInt, 1 Certificate, 1 Name, 4 seq expect 1
// trace:checks GOJ-JWK-ALG
run JwkAlgNeedsPublished for 24 but 1 KeySet, 1 JWT, 1 Verification, 1 JWK, 1 Element, 2 PublicKey, 2 UInt, 1 Certificate, 1 Name, 4 seq expect 1
// trace:checks GOJ-X5C-KEY
run X5cKeyNeedsPublished for 24 but 1 KeySet, 1 JWT, 1 Verification, 1 JWK, 1 Element, 2 PublicKey, 2 UInt, 2 Certificate, 1 Name, 4 seq expect 1
// trace:checks GOJ-JWKS-FORMAT
run JwksFormatNeedsPublished for 24 but 1 KeySet, 1 JWT, 1 Verification, 1 JWK, 1 Element, 1 PublicKey, 1 UInt, 1 Certificate, 1 Name, 4 seq expect 1
// trace:checks GOJ-JWKS-SECRET
run JwksSecretNeedsPublished for 24 but 1 KeySet, 1 JWT, 1 Verification, 1 JWK, 1 Element, 1 PublicKey, 1 UInt, 1 Certificate, 1 Name, 4 seq expect 1
// trace:checks GOJ-JWKS-USABLE
run JwksUsableNeedsPublished for 24 but 1 KeySet, 1 JWT, 1 Verification, 1 JWK, 1 Element, 1 PublicKey, 1 UInt, 1 Certificate, 1 Name, 4 seq expect 1
// trace:checks GOJ-JWKS-KID
run JwksKidNeedsPublished for 24 but 1 KeySet, 1 JWT, 1 Verification, 2 JWK, 2 Element, 3 PublicKey, 3 UInt, 1 Certificate, 1 Name, 4 seq expect 1

// ---- Examples: the Published JWK Set and the standard layer ----

assert GitHubKeySetMeetsStandard {
  all s: KeySet | ghPublished[s] implies {
    stdJwksKeysRequired[s] and stdJwksMemberNameUnique[s] and stdKidDistinct[s] and stdJwksNoPrivate[s]
    all j: jwks[s] {
      stdJwkMemberNameUnique[j] and stdKtyRequired[j] and stdRsaPublicMembers[j]
      stdBase64urlUIntMinimal[j.n] and stdBase64urlUIntMinimal[j.e] and stdRsKeySize[keyOf[j], RS256]
      stdX5cBareKeyPresent[j] and stdX5cBareKeyMatch[j]
    }
  }
}

// trace:checks GH-OIDC-JWKS-RSA-MEMBERS GH-OIDC-JWKS-MODULUS GH-OIDC-JWKS-X5C GH-OIDC-JWKS-X5C-KEY GH-OIDC-JWKS-KID-FORMAT GH-OIDC-JWKS-KID-DISTINCT RFC7517-JWKS-KEYS-REQUIRED RFC7517-JWKS-MEMBER-NAME-UNIQUE RFC7517-KID-DISTINCT RFC7517-MEMBER-NAME-UNIQUE RFC7517-KTY-REQUIRED RFC7518-RSA-PUBLIC-MEMBERS RFC7518-BASE64URLUINT-MINIMAL RFC7518-RS-KEY-SIZE OIDCD-JWKS-NO-PRIVATE OIDCD-JWKS-X5C-BARE-KEY-PRESENT OIDCD-JWKS-X5C-BARE-KEY-MATCH
check GitHubKeySetMeetsStandard for 24 but 1 KeySet, 1 JWT, 1 Verification, 4 JWK, 4 Element, 5 PublicKey, 5 UInt, 3 Certificate, 1 Name, 4 seq expect 0
