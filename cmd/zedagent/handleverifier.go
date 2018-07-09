// Copyright (c) 2017 Zededa, Inc.
// All rights reserved.

package zedagent

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zededa/go-provision/pubsub"
	"github.com/zededa/go-provision/types"
	"log"
	"os"
)

// zedagent is the publishes for these config files
var verifierConfigMap map[string]types.VerifyImageConfig

// zedagent is the subscriber for these status files
var verifierStatusMap map[string]types.VerifyImageStatus

func initVerifierMaps() {

	if verifierConfigMap == nil {
		log.Printf("create verifierConfig map\n")
		verifierConfigMap = make(map[string]types.VerifyImageConfig)
	}

	if verifierStatusMap == nil {
		log.Printf("create verifierStatus map\n")
		verifierStatusMap = make(map[string]types.VerifyImageStatus)
	}
}

func verifierConfigGet(key string) *types.VerifyImageConfig {
	if config, ok := verifierConfigMap[key]; ok {
		log.Printf("%s, verifier config exists, refcount %d\n",
			key, config.RefCount)
		return &config
	}
	log.Printf("%s, verifier config is absent\n", key)
	return nil
}

func verifierConfigGetSha256(sha string) *types.VerifyImageConfig {
	log.Printf("verifierConfigGetSha256(%s)\n", sha)
	for key, config := range verifierConfigMap {
		if config.ImageSha256 == sha {
			log.Printf("verifierConfigGetSha256(%s): found key %s safename %s, refcount %d\n",
				sha, key, config.Safename, config.RefCount)
			return &config
		}
	}
	log.Printf("verifierConfigGetSha256(%s): not found\n", sha)
	return nil
}

func verifierConfigSet(key string, config *types.VerifyImageConfig) {
	verifierConfigMap[key] = *config
}

func verifierStatusGet(key string) *types.VerifyImageStatus {
	if status, ok := verifierStatusMap[key]; ok {
		return &status
	}
	log.Printf("%s, verifier status is absent\n", key)
	return nil
}

func verifierStatusSet(key string, status *types.VerifyImageStatus) {
	verifierStatusMap[key] = *status
}

func verifierStatusDelete(key string) {
	log.Printf("%s, verifier status entry delete\n", key)
	if status := verifierStatusGet(key); status != nil {
		delete(verifierStatusMap, key)
	}
}

// If checkCerts is set this can return an error. Otherwise not.
func createVerifierConfig(objType string, safename string,
	sc *types.StorageConfig, checkCerts bool) error {

	initVerifierMaps()

	// check the certificate files, if not present,
	// we can not start verification
	if checkCerts {
		if err := checkCertsForObject(safename, sc); err != nil {
			log.Printf("%v for %s\n", err, safename)
			return err
		}
	}

	key := formLookupKey(objType, safename)

	if m := verifierConfigGet(key); m != nil {
		m.RefCount += 1
		verifierConfigSet(key, m)
	} else {
		log.Printf("%s, verifier config add\n", safename)
		n := types.VerifyImageConfig{
			Safename:         safename,
			DownloadURL:      sc.DownloadURL,
			ImageSha256:      sc.ImageSha256,
			CertificateChain: sc.CertificateChain,
			ImageSignature:   sc.ImageSignature,
			SignatureKey:     sc.SignatureKey,
			RefCount:         1,
		}
		verifierConfigSet(key, &n)
	}

	writeVerifierConfig(objType, safename, verifierConfigGet(key))

	log.Printf("%s, createVerifierConfig done\n", safename)
	return nil
}

func updateVerifierStatus(ctx *zedagentContext, objType string,
	status *types.VerifyImageStatus) {

	initVerifierMaps()

	key := formLookupKey(objType, status.Safename)
	log.Printf("%s, updateVerifierStatus\n", key)

	// Ignore if any Pending* flag is set
	if status.PendingAdd || status.PendingModify || status.PendingDelete {
		log.Printf("%s, Skipping due to Pending*\n", key)
		return
	}

	changed := false
	if m := verifierStatusGet(key); m != nil {
		if status.State != m.State {
			log.Printf("%s, verifier entry change, State %v to %v\n",
				key, m.State, status.State)
			changed = true
		} else if status.Size != m.Size {
			log.Printf("%s, verifier entry change, Size %v to %v\n",
				key, m.Size, status.Size)
			changed = true
		} else {
			log.Printf("%s, verifier entry no change, State %v\n",
				key, status.State)
		}
	} else {
		log.Printf("%s, verifier status entry add, State %v\n", key, status.State)
		changed = true

	}

	if changed {
		verifierStatusSet(key, status)
		if objType == baseOsObj {
			baseOsHandleStatusUpdateSafename(ctx, status.Safename)
		}
	}

	log.Printf("updateVerifierStatus for %s, Done\n", key)
}

func MaybeRemoveVerifierConfigSha256(objType string, sha256 string) {
	log.Printf("MaybeRemoveVerifierConfig for %s\n", sha256)

	m := verifierConfigGetSha256(sha256)
	if m == nil {
		log.Printf("Verifier config missing for remove for %s\n",
			sha256)
		return
	}
	safename := m.Safename
	key := formLookupKey(objType, safename)
	log.Printf("MaybeRemoveVerifierConfig key %s\n", key)

	m.RefCount -= 1
	if m.RefCount != 0 {
		log.Printf("MaybeRemoveVerifierConfig remaining RefCount %d for %s\n",
			m.RefCount, sha256)
		verifierConfigSet(key, m)
		writeVerifierConfig(objType, safename, m)
		return
	}
	delete(verifierConfigMap, key)
	deleteVerifierConfig(objType, safename)
	log.Printf("MaybeRemoveVerifierConfigSha256 done for %s\n", sha256)
}

func removeVerifierStatus(ctx *zedagentContext, objType string, safename string) {

	key := formLookupKey(objType, safename)
	verifierStatusDelete(key)
}

func lookupVerificationStatusSha256(objType string, sha256 string) (*types.VerifyImageStatus, error) {

	for _, status := range verifierStatusMap {
		if status.ImageSha256 == sha256 {
			return &status, nil
		}
	}

	return nil, errors.New("No verificationStatusMap for sha")
}

func lookupVerificationStatus(objType string, safename string) (*types.VerifyImageStatus, error) {

	key := formLookupKey(objType, safename)

	if m := verifierStatusGet(key); m != nil {
		log.Printf("lookupVerifyImageStatus: found based on safename %s\n",
			safename)
		return m, nil
	}
	return nil, errors.New("No verificationStatusMap for safename")
}

func lookupVerificationStatusAny(objType string, safename string, sha256 string) (*types.VerifyImageStatus, error) {

	if m, err := lookupVerificationStatus(objType, safename); err == nil {
		return m, nil
	}
	if m, err := lookupVerificationStatusSha256(objType, sha256); err == nil {
		log.Printf("lookupVerifyImageStatusAny: found based on sha %s\n", sha256)
		return m, nil
	}
	return nil, errors.New("No verification status for safename nor sha")
}

func checkStorageVerifierStatus(objType string, uuidStr string,
	config []types.StorageConfig, status []types.StorageStatus) *types.RetStatus {

	ret := &types.RetStatus{}
	key := formLookupKey(objType, uuidStr)

	log.Printf("%s, checkStorageVerifierStatus\n", key)

	ret.AllErrors = ""
	ret.Changed = false
	ret.MinState = types.MAXSTATE

	for i, sc := range config {
		ss := &status[i]

		safename := types.UrlToSafename(sc.DownloadURL, sc.ImageSha256)

		log.Printf("%s, image verifier status %v\n", sc.DownloadURL, ss.State)

		if ss.State == types.INSTALLED {
			ret.MinState = ss.State
			continue
		}

		vs, err := lookupVerificationStatusAny(objType, safename, sc.ImageSha256)
		if err != nil {
			log.Printf("%s, %v\n", safename, err)
			ret.MinState = types.DOWNLOADED
			continue
		}
		if ret.MinState > vs.State {
			ret.MinState = vs.State
		}
		if vs.State != ss.State {
			log.Printf("checkStorageVerifierStatus(%s) set ss.State %d\n",
				safename, vs.State)
			ss.State = vs.State
			ret.Changed = true
		}
		switch vs.State {
		case types.INITIAL:
			log.Printf("%s, verifier error for %s: %s\n",
				key, safename, vs.LastErr)
			ss.Error = vs.LastErr
			ret.AllErrors = appendError(ret.AllErrors, "verifier",
				vs.LastErr)
			ss.ErrorTime = vs.LastErrTime
			ret.ErrorTime = vs.LastErrTime
			ret.Changed = true
		default:
			ss.ActiveFileLocation = objectDownloadDirname + "/" + objType + "/" + vs.Safename

			log.Printf("%s, Update SSL ActiveFileLocation for %s: %s\n",
				key, uuidStr, ss.ActiveFileLocation)
			ret.Changed = true
		}
	}

	if ret.MinState == types.MAXSTATE {
		// Odd; no StorageConfig in list
		ret.MinState = types.DELIVERED
	}
	return ret
}

func writeVerifierConfig(objType string, safename string,
	config *types.VerifyImageConfig) {
	if config == nil {
		return
	}
	log.Printf("writeVerifierConfig(%s): RefCount %d\n",
		safename, config.RefCount)
	configFilename := fmt.Sprintf("%s/%s/config/%s.json",
		verifierBaseDirname, objType, safename)

	bytes, err := json.Marshal(config)
	if err != nil {
		log.Fatal(err, "json Marshal VerifyImageConfig")
	}

	err = pubsub.WriteRename(configFilename, bytes)
	if err != nil {
		log.Fatal(err)
	}
}

func deleteVerifierConfig(objType string, safename string) {
	log.Printf("deleteVerifierConfig(%s)\n", safename)
	configFilename := fmt.Sprintf("%s/%s/config/%s.json",
		verifierBaseDirname, objType, safename)
	if err := os.Remove(configFilename); err != nil {
		log.Println(err)
	}
}

// check whether the cert files are installed
func checkCertsForObject(safename string, sc *types.StorageConfig) error {
	var cidx int = 0

	// count the number of cerificates in this object
	if sc.SignatureKey != "" {
		cidx++
	}
	for _, certUrl := range sc.CertificateChain {
		if certUrl != "" {
			cidx++
		}
	}
	// if no cerificates, return
	if cidx == 0 {
		log.Printf("%s, checkCertsForObject, no configured certificates\n",
			safename)
		return nil
	}

	if sc.SignatureKey != "" {
		safename := types.UrlToSafename(sc.SignatureKey, "")
		filename := certificateDirname + "/" +
			types.SafenameToFilename(safename)
		if _, err := os.Stat(filename); err != nil {
			log.Printf("%s, checkCertsForObject %v\n", filename, err)
			return err
		}
	}

	for _, certUrl := range sc.CertificateChain {
		if certUrl != "" {
			safename := types.UrlToSafename(certUrl, "")
			filename := certificateDirname + "/" +
				types.SafenameToFilename(safename)
			if _, err := os.Stat(filename); err != nil {
				log.Printf("%s, checkCertsForObject %v\n", filename, err)
				return err
			}
		}
	}
	return nil
}
