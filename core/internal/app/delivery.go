package app

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"autosecrets.dev/core/internal/envelope"
	"autosecrets.dev/core/internal/server"
	"autosecrets.dev/core/internal/store"
	"filippo.io/age"
)

// desiredResponse is what a node receives on polling: a stable ETag plus one
// encrypted, signed envelope per assigned Bundle Revision.
type desiredResponse struct {
	ETag      string              `json:"etag"`
	Envelopes []*envelope.Envelope `json:"envelopes"`
}

// handleDesired serves the node's complete Desired State. A 304 is returned
// when If-None-Match matches the current ETag. The envelope is encrypted to
// the node's age public key and signed by Core (ADR-0011).
func (a *App) handleDesired(w http.ResponseWriter, r *http.Request) {
	node, err := a.nodeFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "unknown node identity"})
		return
	}
	revisionIDs, err := a.store.AssignedRevisions(r.Context(), node.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	etag := etagOf(revisionIDs)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_ = a.store.SetNodeDesired(r.Context(), node.ID, etag)
	if len(revisionIDs) == 0 {
		writeJSON(w, http.StatusOK, desiredResponse{ETag: etag, Envelopes: []*envelope.Envelope{}})
		return
	}
	recipient, err := age.ParseX25519Recipient(node.AgePubkey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	envs := make([]*envelope.Envelope, 0, len(revisionIDs))
	for _, revisionID := range revisionIDs {
		env, err := a.buildEnvelope(r, node.ID, revisionID, recipient)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		envs = append(envs, env)
	}
	writeJSON(w, http.StatusOK, desiredResponse{ETag: etag, Envelopes: envs})
}

func (a *App) buildEnvelope(r *http.Request, nodeID, revisionID string, recipient *age.X25519Recipient) (*envelope.Envelope, error) {
	files, err := a.store.RevisionFiles(r.Context(), revisionID)
	if err != nil {
		return nil, err
	}
	appID, envID, err := a.store.RevisionAppEnv(r.Context(), revisionID)
	if err != nil {
		return nil, err
	}
	type payloadFile struct {
		Path    string `json:"path"`
		Mode    string `json:"mode"`
		UID     string `json:"uid"`
		GID     string `json:"gid"`
		Content string `json:"content"`
	}
	payload := struct {
		AppID string        `json:"app_id"`
		EnvID string        `json:"env_id"`
		Files []payloadFile `json:"files"`
	}{AppID: appID, EnvID: envID, Files: []payloadFile{}}
	manifestFiles := []envelope.FileSpec{}
	for _, f := range files {
		wrapped, nonces, ct, err := a.store.SecretVersionBlob(r.Context(), f.SecretID, f.VersionSeq)
		if err != nil {
			return nil, err
		}
		plain, err := a.mk.Open(wrapped, nonces, ct)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(plain)
		manifestFiles = append(manifestFiles, envelope.FileSpec{
			Path:   f.Path,
			Mode:   modeString(f.Mode),
			UID:    strconv.FormatInt(f.UID, 10),
			GID:    strconv.FormatInt(f.GID, 10),
			SHA256: hex.EncodeToString(sum[:]),
		})
		payload.Files = append(payload.Files, payloadFile{
			Path:    f.Path,
			Mode:    modeString(f.Mode),
			UID:     strconv.FormatInt(f.UID, 10),
			GID:     strconv.FormatInt(f.GID, 10),
			Content: base64.StdEncoding.EncodeToString(plain),
		})
	}
	manifestBytes, err := envelope.CanonicalManifest(manifestFiles)
	if err != nil {
		return nil, err
	}
	plaintextBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return envelope.New(envelope.Options{
		NodeID:       nodeID,
		RevisionID:   revisionID,
		CreatedAt:    a.now(),
		ExpiresAt:    a.now().Add(10 * time.Minute),
		Manifest:     manifestBytes,
		Plaintext:    plaintextBytes,
		Recipient:    recipient,
		Signer:       a.signer.PrivateKey(),
		SigningKeyID: a.signer.KeyID(),
	})
}

// nodeFromRequest resolves the Managed Node behind the forwarded client
// certificate serial.
func (a *App) nodeFromRequest(r *http.Request) (*store.Node, error) {
	serial, ok := server.AgentSerialFromContext(r.Context())
	if !ok {
		return nil, errors.New("no node identity")
	}
	return a.store.NodeBySerial(r.Context(), serial)
}

func (a *App) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	node, err := a.nodeFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "unknown node identity"})
		return
	}
	if node.ID != r.PathValue("nodeID") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "node identity mismatch"})
		return
	}
	if err := a.store.TouchNode(r.Context(), node.ID, node.ObservedRevision, node.LastResult); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleReport(w http.ResponseWriter, r *http.Request) {
	node, err := a.nodeFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "unknown node identity"})
		return
	}
	if node.ID != r.PathValue("nodeID") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "node identity mismatch"})
		return
	}
	var body struct {
		RevisionID string `json:"revision_id"`
		Stage      string `json:"stage"`
		Result     string `json:"result"`
		Error      string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Result == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "result is required"})
		return
	}
	lastResult := body.Result
	if body.Error != "" {
		lastResult += ": " + body.Error
	}
	if err := a.store.TouchNode(r.Context(), node.ID, body.RevisionID, lastResult); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	_ = a.store.AppendAudit(r.Context(), nil, store.AuditEvent{
		Actor: "node:" + node.Name, Action: "activation." + body.Stage, Resource: body.RevisionID,
		Result: body.Result, CorrelationID: a.correlationID(r),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

func etagOf(revisionIDs []string) string {
	sorted := append([]string(nil), revisionIDs...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	return `"` + hex.EncodeToString(sum[:]) + `"`
}
