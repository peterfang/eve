// Copyright (c) 2020 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0

// Process input in the form of a collection of VolumeConfig structs
// from zedmanager and baseosmgr. Publish the status as VolumeStatus

package volumemgr

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/lf-edge/eve/pkg/pillar/agentlog"
	"github.com/lf-edge/eve/pkg/pillar/containerd"
	"github.com/lf-edge/eve/pkg/pillar/pidfile"
	"github.com/lf-edge/eve/pkg/pillar/pubsub"
	"github.com/lf-edge/eve/pkg/pillar/types"
	"github.com/lf-edge/eve/pkg/pillar/worker"
	log "github.com/sirupsen/logrus"
)

const (
	agentName              = "volumemgr"
	runDirname             = "/var/run/" + agentName
	ciDirname              = runDirname + "/cloudinit"    // For cloud-init volumes XXX change?
	rwImgDirname           = types.RWImgDirname           // We store old VM volumes here
	roContImgDirname       = types.ROContImgDirname       // We store old OCI volumes here
	volumeEncryptedDirName = types.VolumeEncryptedDirName // We store VM and OCI volumes here
	// Time limits for event loop handlers
	errorTime   = 3 * time.Minute
	warningTime = 40 * time.Second
)

// Set from Makefile
var Version = "No version specified"
var downloadGCTime = time.Duration(600) * time.Second // Unless from GlobalConfig

type volumemgrContext struct {
	subAppVolumeConfig        pubsub.Subscription
	pubAppVolumeStatus        pubsub.Publication
	subBaseOsVolumeConfig     pubsub.Subscription
	pubBaseOsVolumeStatus     pubsub.Publication
	pubUnknownOldVolumeStatus pubsub.Publication
	subGlobalConfig           pubsub.Subscription
	subZedAgentStatus         pubsub.Subscription

	subCertObjConfig         pubsub.Subscription
	pubCertObjStatus         pubsub.Publication
	pubCertObjDownloadConfig pubsub.Publication
	subCertObjDownloadStatus pubsub.Subscription

	pubAppImgDownloadConfig pubsub.Publication
	subAppImgDownloadStatus pubsub.Subscription
	pubAppImgVerifierConfig pubsub.Publication
	subAppImgVerifierStatus pubsub.Subscription
	pubBaseOsDownloadConfig pubsub.Publication
	subBaseOsDownloadStatus pubsub.Subscription
	pubBaseOsVerifierConfig pubsub.Publication
	subBaseOsVerifierStatus pubsub.Subscription
	pubAppImgPersistStatus  pubsub.Publication
	subAppImgPersistStatus  pubsub.Subscription
	pubBaseOsPersistStatus  pubsub.Publication
	subBaseOsPersistStatus  pubsub.Subscription

	subContentTreeResolveStatus pubsub.Subscription
	pubContentTreeResolveConfig pubsub.Publication
	subContentTreeConfig        pubsub.Subscription
	pubContentTreeStatus        pubsub.Publication
	subVolumeConfig             pubsub.Subscription
	pubVolumeStatus             pubsub.Publication
	pubUnknownVolumeStatus      pubsub.Publication
	pubContentTreeToHash        pubsub.Publication

	gc *time.Ticker

	workerOld *worker.Worker // For background work
	worker    *worker.Worker // For background work

	verifierRestarted uint // Count to two for appimg and baseos
	usingConfig       bool // From zedagent
	gcRunning         bool

	globalConfig  *types.ConfigItemValueMap
	GCInitialized bool
	vdiskGCTime   uint32 // In seconds
}

var debug = false
var debugOverride bool // From command line arg

// Run - the main function invoked by zedbox
func Run(ps *pubsub.PubSub) {
	versionPtr := flag.Bool("v", false, "Version")
	debugPtr := flag.Bool("d", false, "Debug flag")
	flag.Parse()
	debug = *debugPtr
	debugOverride = debug
	if debugOverride {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	if *versionPtr {
		fmt.Printf("%s: %s\n", os.Args[0], Version)
		return
	}
	agentlog.Init(agentName)

	if err := pidfile.CheckAndCreatePidfile(agentName); err != nil {
		log.Fatal(err)
	}
	log.Infof("Starting %s", agentName)

	// create the directories
	initializeDirs()

	// These settings can be overridden by GlobalConfig
	ctx := volumemgrContext{
		vdiskGCTime:  3600,
		globalConfig: types.DefaultConfigItemValueMap(),
	}

	// Run a periodic timer so we always update StillRunning
	stillRunning := time.NewTicker(25 * time.Second)
	agentlog.StillRunning(agentName, warningTime, errorTime)

	// Look for global config such as log levels
	subGlobalConfig, err := ps.NewSubscription(pubsub.SubscriptionOptions{
		AgentName:     "",
		TopicImpl:     types.ConfigItemValueMap{},
		Activate:      false,
		Ctx:           &ctx,
		CreateHandler: handleGlobalConfigModify,
		ModifyHandler: handleGlobalConfigModify,
		DeleteHandler: handleGlobalConfigDelete,
		WarningTime:   warningTime,
		ErrorTime:     errorTime,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subGlobalConfig = subGlobalConfig
	subGlobalConfig.Activate()

	// Create the background worker
	ctx.workerOld = InitHandleWorkOld(&ctx)
	ctx.worker = InitHandleWork(&ctx)

	// Set up our publications before the subscriptions so ctx is set
	pubAppImgDownloadConfig, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		AgentScope: types.AppImgObj,
		TopicType:  types.DownloaderConfig{},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubAppImgDownloadConfig = pubAppImgDownloadConfig

	pubAppImgVerifierConfig, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		AgentScope: types.AppImgObj,
		TopicType:  types.VerifyImageConfig{},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubAppImgVerifierConfig = pubAppImgVerifierConfig

	pubBaseOsDownloadConfig, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		AgentScope: types.BaseOsObj,
		TopicType:  types.DownloaderConfig{},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubBaseOsDownloadConfig = pubBaseOsDownloadConfig

	pubBaseOsVerifierConfig, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		AgentScope: types.BaseOsObj,
		TopicType:  types.VerifyImageConfig{},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubBaseOsVerifierConfig = pubBaseOsVerifierConfig

	pubContentTreeResolveConfig, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		AgentScope: types.AppImgObj,
		TopicType:  types.ResolveConfig{},
	})
	if err != nil {
		log.Fatal(err)
	}
	pubContentTreeResolveConfig.ClearRestarted()
	ctx.pubContentTreeResolveConfig = pubContentTreeResolveConfig

	pubContentTreeStatus, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		AgentScope: types.AppImgObj,
		TopicType:  types.ContentTreeStatus{},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubContentTreeStatus = pubContentTreeStatus

	pubVolumeStatus, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		AgentScope: types.AppImgObj,
		TopicType:  types.VolumeStatus{},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubVolumeStatus = pubVolumeStatus

	pubUnknownVolumeStatus, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		AgentScope: types.UnknownObj,
		TopicType:  types.VolumeStatus{},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubUnknownVolumeStatus = pubUnknownVolumeStatus

	pubContentTreeToHash, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		Persistent: true,
		TopicType:  types.AppAndImageToHash{},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubContentTreeToHash = pubContentTreeToHash

	pubAppVolumeStatus, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		AgentScope: types.AppImgObj,
		TopicType:  types.OldVolumeStatus{},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubAppVolumeStatus = pubAppVolumeStatus

	pubBaseOsVolumeStatus, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		AgentScope: types.BaseOsObj,
		TopicType:  types.OldVolumeStatus{},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubBaseOsVolumeStatus = pubBaseOsVolumeStatus

	pubUnknownOldVolumeStatus, err := ps.NewPublication(pubsub.PublicationOptions{
		AgentName:  agentName,
		AgentScope: types.UnknownObj,
		TopicType:  types.OldVolumeStatus{},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubUnknownOldVolumeStatus = pubUnknownOldVolumeStatus

	pubCertObjStatus, err := ps.NewPublication(
		pubsub.PublicationOptions{
			AgentName: agentName,
			TopicType: types.CertObjStatus{},
		})

	if err != nil {
		log.Fatal(err)
	}
	pubCertObjStatus.ClearRestarted()
	ctx.pubCertObjStatus = pubCertObjStatus

	pubCertObjDownloadConfig, err := ps.NewPublication(
		pubsub.PublicationOptions{
			AgentName:  agentName,
			AgentScope: types.CertObj,
			TopicType:  types.DownloaderConfig{},
		})
	if err != nil {
		log.Fatal(err)
	}
	pubCertObjDownloadConfig.ClearRestarted()
	ctx.pubCertObjDownloadConfig = pubCertObjDownloadConfig

	// Set up our publications before the subscriptions so ctx is set
	pubAppImgPersistStatus, err := ps.NewPublication(
		pubsub.PublicationOptions{
			AgentName:  agentName,
			AgentScope: types.AppImgObj,
			TopicType:  types.PersistImageStatus{},
		})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubAppImgPersistStatus = pubAppImgPersistStatus

	pubBaseOsPersistStatus, err := ps.NewPublication(
		pubsub.PublicationOptions{
			AgentName:  agentName,
			AgentScope: types.BaseOsObj,
			TopicType:  types.PersistImageStatus{},
		})
	if err != nil {
		log.Fatal(err)
	}
	ctx.pubBaseOsPersistStatus = pubBaseOsPersistStatus

	// Publish existing volumes with RefCount zero in the "unknown"
	// agentScope
	// Note that nobody subscribes to this. Used internally
	// to look up a Volume.
	populateInitialOldVolumeStatus(&ctx, rwImgDirname)
	populateInitialOldVolumeStatus(&ctx, roContImgDirname)
	populateInitialVolumeStatus(&ctx, volumeEncryptedDirName)

	// Look for global config such as log levels
	subZedAgentStatus, err := ps.NewSubscription(pubsub.SubscriptionOptions{
		AgentName:     "zedagent",
		TopicImpl:     types.ZedAgentStatus{},
		Activate:      false,
		Ctx:           &ctx,
		CreateHandler: handleZedAgentStatusModify,
		ModifyHandler: handleZedAgentStatusModify,
		WarningTime:   warningTime,
		ErrorTime:     errorTime,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subZedAgentStatus = subZedAgentStatus
	subZedAgentStatus.Activate()

	// Look for DownloaderStatus from downloader
	subAppImgDownloadStatus, err := ps.NewSubscription(pubsub.SubscriptionOptions{
		AgentName:     "downloader",
		AgentScope:    types.AppImgObj,
		TopicImpl:     types.DownloaderStatus{},
		Activate:      false,
		Ctx:           &ctx,
		CreateHandler: handleDownloaderStatusModify,
		ModifyHandler: handleDownloaderStatusModify,
		DeleteHandler: handleDownloaderStatusDelete,
		WarningTime:   warningTime,
		ErrorTime:     errorTime,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subAppImgDownloadStatus = subAppImgDownloadStatus
	subAppImgDownloadStatus.Activate()

	// Look for VerifyImageStatus from verifier
	subAppImgVerifierStatus, err := ps.NewSubscription(pubsub.SubscriptionOptions{
		AgentName:      "verifier",
		AgentScope:     types.AppImgObj,
		TopicImpl:      types.VerifyImageStatus{},
		Activate:       false,
		Ctx:            &ctx,
		CreateHandler:  handleVerifyImageStatusModify,
		ModifyHandler:  handleVerifyImageStatusModify,
		DeleteHandler:  handleVerifyImageStatusDelete,
		RestartHandler: handleVerifierRestarted,
		WarningTime:    warningTime,
		ErrorTime:      errorTime,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subAppImgVerifierStatus = subAppImgVerifierStatus
	subAppImgVerifierStatus.Activate()

	// Look for PersistImageStatus from verifier
	subAppImgPersistStatus, err := ps.NewSubscription(
		pubsub.SubscriptionOptions{
			AgentName:     "verifier",
			AgentScope:    types.AppImgObj,
			TopicImpl:     types.PersistImageStatus{},
			Activate:      false,
			Ctx:           &ctx,
			CreateHandler: handlePersistImageStatusCreate,
			WarningTime:   warningTime,
			ErrorTime:     errorTime,
		})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subAppImgPersistStatus = subAppImgPersistStatus
	subAppImgPersistStatus.Activate()

	// Look for PersistImageStatus from verifier
	subBaseOsPersistStatus, err := ps.NewSubscription(
		pubsub.SubscriptionOptions{
			AgentName:     "verifier",
			AgentScope:    types.BaseOsObj,
			TopicImpl:     types.PersistImageStatus{},
			Activate:      false,
			Ctx:           &ctx,
			CreateHandler: handlePersistImageStatusCreate,
			WarningTime:   warningTime,
			ErrorTime:     errorTime,
		})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subBaseOsPersistStatus = subBaseOsPersistStatus
	subBaseOsPersistStatus.Activate()

	// Look for DownloaderStatus from downloader
	subBaseOsDownloadStatus, err := ps.NewSubscription(pubsub.SubscriptionOptions{
		AgentName:     "downloader",
		AgentScope:    types.BaseOsObj,
		TopicImpl:     types.DownloaderStatus{},
		Activate:      false,
		Ctx:           &ctx,
		CreateHandler: handleDownloaderStatusModify,
		ModifyHandler: handleDownloaderStatusModify,
		DeleteHandler: handleDownloaderStatusDelete,
		WarningTime:   warningTime,
		ErrorTime:     errorTime,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subBaseOsDownloadStatus = subBaseOsDownloadStatus
	subBaseOsDownloadStatus.Activate()

	// Look for VerifyImageStatus from verifier
	subBaseOsVerifierStatus, err := ps.NewSubscription(pubsub.SubscriptionOptions{
		AgentName:      "verifier",
		AgentScope:     types.BaseOsObj,
		TopicImpl:      types.VerifyImageStatus{},
		Activate:       false,
		Ctx:            &ctx,
		CreateHandler:  handleVerifyImageStatusModify,
		ModifyHandler:  handleVerifyImageStatusModify,
		DeleteHandler:  handleVerifyImageStatusDelete,
		RestartHandler: handleVerifierRestarted,
		WarningTime:    warningTime,
		ErrorTime:      errorTime,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subBaseOsVerifierStatus = subBaseOsVerifierStatus
	subBaseOsVerifierStatus.Activate()

	// Look for CertObjConfig, from zedagent
	subCertObjConfig, err := ps.NewSubscription(
		pubsub.SubscriptionOptions{
			AgentName:     "zedagent",
			TopicImpl:     types.CertObjConfig{},
			Activate:      false,
			Ctx:           &ctx,
			CreateHandler: handleCertObjCreate,
			ModifyHandler: handleCertObjModify,
			DeleteHandler: handleCertObjConfigDelete,
			WarningTime:   warningTime,
			ErrorTime:     errorTime,
		})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subCertObjConfig = subCertObjConfig
	subCertObjConfig.Activate()

	// Look for Certs DownloaderStatus from downloader
	subCertObjDownloadStatus, err := ps.NewSubscription(
		pubsub.SubscriptionOptions{
			AgentName:     "downloader",
			AgentScope:    types.CertObj,
			TopicImpl:     types.DownloaderStatus{},
			Activate:      false,
			Ctx:           &ctx,
			CreateHandler: handleDownloadStatusModify,
			ModifyHandler: handleDownloadStatusModify,
			DeleteHandler: handleDownloadStatusDelete,
			WarningTime:   warningTime,
			ErrorTime:     errorTime,
		})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subCertObjDownloadStatus = subCertObjDownloadStatus
	subCertObjDownloadStatus.Activate()

	// Look for ContentTreeResolveStatus from downloader
	subContentTreeResolveStatus, err := ps.NewSubscription(pubsub.SubscriptionOptions{
		AgentName:     "downloader",
		AgentScope:    types.AppImgObj,
		TopicImpl:     types.ResolveStatus{},
		Activate:      false,
		Ctx:           &ctx,
		CreateHandler: handleResolveStatusModify,
		ModifyHandler: handleResolveStatusModify,
		WarningTime:   warningTime,
		ErrorTime:     errorTime,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subContentTreeResolveStatus = subContentTreeResolveStatus
	subContentTreeResolveStatus.Activate()

	subContentTreeConfig, err := ps.NewSubscription(pubsub.SubscriptionOptions{
		CreateHandler: handleContentTreeCreate,
		ModifyHandler: handleContentTreeModify,
		DeleteHandler: handleContentTreeDelete,
		WarningTime:   warningTime,
		ErrorTime:     errorTime,
		AgentName:     "zedagent",
		TopicImpl:     types.ContentTreeConfig{},
		Ctx:           &ctx,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subContentTreeConfig = subContentTreeConfig
	subContentTreeConfig.Activate()

	subVolumeConfig, err := ps.NewSubscription(pubsub.SubscriptionOptions{
		CreateHandler: handleVolumeCreate,
		ModifyHandler: handleVolumeModify,
		DeleteHandler: handleVolumeDelete,
		WarningTime:   warningTime,
		ErrorTime:     errorTime,
		AgentName:     "zedagent",
		TopicImpl:     types.VolumeConfig{},
		Ctx:           &ctx,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subVolumeConfig = subVolumeConfig
	subVolumeConfig.Activate()

	subAppVolumeConfig, err := ps.NewSubscription(pubsub.SubscriptionOptions{
		CreateHandler: handleAppImgCreate,
		ModifyHandler: handleAppImgModify,
		DeleteHandler: handleAppImgDelete,
		WarningTime:   warningTime,
		ErrorTime:     errorTime,
		AgentName:     "zedmanager",
		AgentScope:    types.AppImgObj,
		TopicImpl:     types.OldVolumeConfig{},
		Ctx:           &ctx,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subAppVolumeConfig = subAppVolumeConfig
	subAppVolumeConfig.Activate()

	subBaseOsVolumeConfig, err := ps.NewSubscription(pubsub.SubscriptionOptions{
		CreateHandler: handleBaseOsCreate,
		ModifyHandler: handleBaseOsModify,
		DeleteHandler: handleBaseOsDelete,
		WarningTime:   warningTime,
		ErrorTime:     errorTime,
		AgentName:     "baseosmgr",
		AgentScope:    types.BaseOsObj,
		TopicImpl:     types.OldVolumeConfig{},
		Ctx:           &ctx,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx.subBaseOsVolumeConfig = subBaseOsVolumeConfig
	subBaseOsVolumeConfig.Activate()

	// Pick up debug aka log level before we start real work
	for !ctx.GCInitialized {
		log.Infof("waiting for GCInitialized")
		select {
		case change := <-subGlobalConfig.MsgChan():
			subGlobalConfig.ProcessChange(change)
		case <-stillRunning.C:
		}
		agentlog.StillRunning(agentName, warningTime, errorTime)
	}
	log.Infof("processed GlobalConfig")

	if err := containerd.InitContainerdClient(); err != nil {
		log.Fatal(err)
	}
	defer containerd.CtrdClient.Close()

	// First we process the verifierStatus to avoid triggering a download
	// of an image we already have in place.
	for ctx.verifierRestarted != 2 {
		log.Infof("Waiting for verifierRestarted: is %d",
			ctx.verifierRestarted)

		select {
		case change := <-subGlobalConfig.MsgChan():
			subGlobalConfig.ProcessChange(change)

		case change := <-subAppImgVerifierStatus.MsgChan():
			subAppImgVerifierStatus.ProcessChange(change)

		case change := <-subAppImgPersistStatus.MsgChan():
			subAppImgPersistStatus.ProcessChange(change)

		case change := <-subBaseOsVerifierStatus.MsgChan():
			subBaseOsVerifierStatus.ProcessChange(change)

		case res := <-ctx.workerOld.MsgChan():
			HandleWorkResultOld(&ctx, ctx.workerOld.Process(res))

		case res := <-ctx.worker.MsgChan():
			HandleWorkResult(&ctx, ctx.worker.Process(res))

		case <-stillRunning.C:
		}
		agentlog.StillRunning(agentName, warningTime, errorTime)
	}
	log.Infof("Handling all inputs")

	// We will cleanup zero RefCount objects after a while
	// We run timer 10 times more often than the limit on LastUse
	// We start the timer once ZedAgentStatus tells us we are receiving
	// config (or using a saved config) to avoid removing volumes when
	// they might become used.
	duration := time.Duration(ctx.vdiskGCTime / 10)
	ctx.gc = time.NewTicker(duration * time.Second)
	ctx.gc.Stop()

	for {
		select {
		case change := <-ctx.subGlobalConfig.MsgChan():
			ctx.subGlobalConfig.ProcessChange(change)

		case change := <-subZedAgentStatus.MsgChan():
			subZedAgentStatus.ProcessChange(change)
			if ctx.usingConfig && !ctx.gcRunning {
				log.Infof("Starting gc timer")
				duration := time.Duration(ctx.vdiskGCTime / 10)
				ctx.gc = time.NewTicker(duration * time.Second)
				// Update the LastUse here to be now
				gcResetOldObjectsLastUse(&ctx, rwImgDirname)
				gcResetOldObjectsLastUse(&ctx, roContImgDirname)
				gcResetObjectsLastUse(&ctx, volumeEncryptedDirName)
				gcResetPersistObjectLastUse(&ctx)
				ctx.gcRunning = true
			}

		case change := <-ctx.subCertObjConfig.MsgChan():
			ctx.subCertObjConfig.ProcessChange(change)

		case change := <-subAppImgDownloadStatus.MsgChan():
			subAppImgDownloadStatus.ProcessChange(change)

		case change := <-subAppImgVerifierStatus.MsgChan():
			subAppImgVerifierStatus.ProcessChange(change)

		case change := <-subBaseOsDownloadStatus.MsgChan():
			subBaseOsDownloadStatus.ProcessChange(change)

		case change := <-subBaseOsVerifierStatus.MsgChan():
			subBaseOsVerifierStatus.ProcessChange(change)

		case change := <-ctx.subCertObjDownloadStatus.MsgChan():
			ctx.subCertObjDownloadStatus.ProcessChange(change)

		case change := <-subContentTreeResolveStatus.MsgChan():
			ctx.subContentTreeResolveStatus.ProcessChange(change)

		case change := <-ctx.subContentTreeConfig.MsgChan():
			ctx.subContentTreeConfig.ProcessChange(change)

		case change := <-ctx.subVolumeConfig.MsgChan():
			ctx.subVolumeConfig.ProcessChange(change)

		case change := <-ctx.subAppVolumeConfig.MsgChan():
			ctx.subAppVolumeConfig.ProcessChange(change)

		case change := <-ctx.subBaseOsVolumeConfig.MsgChan():
			ctx.subBaseOsVolumeConfig.ProcessChange(change)

		case change := <-subAppImgPersistStatus.MsgChan():
			subAppImgPersistStatus.ProcessChange(change)

		case change := <-subBaseOsPersistStatus.MsgChan():
			subBaseOsPersistStatus.ProcessChange(change)

		case <-ctx.gc.C:
			start := time.Now()
			gcOldObjects(&ctx, rwImgDirname)
			gcOldObjects(&ctx, roContImgDirname)
			gcObjects(&ctx, volumeEncryptedDirName)
			gcVerifiedObjects(&ctx)
			pubsub.CheckMaxTimeTopic(agentName, "gc", start,
				warningTime, errorTime)

		case res := <-ctx.workerOld.MsgChan():
			HandleWorkResultOld(&ctx, ctx.workerOld.Process(res))

		case res := <-ctx.worker.MsgChan():
			HandleWorkResult(&ctx, ctx.worker.Process(res))

		case <-stillRunning.C:
		}
		agentlog.StillRunning(agentName, warningTime, errorTime)
	}
}

// We could since we get a separate out-of-order notification for
// each objType
func handleVerifierRestarted(ctxArg interface{}, done bool) {
	ctx := ctxArg.(*volumemgrContext)

	log.Infof("handleVerifierRestarted(%v)", done)
	if done {
		ctx.verifierRestarted++
	}
}

func handleGlobalConfigModify(ctxArg interface{}, key string,
	statusArg interface{}) {

	ctx := ctxArg.(*volumemgrContext)
	if key != "global" {
		log.Infof("handleGlobalConfigModify: ignoring %s", key)
		return
	}
	log.Infof("handleGlobalConfigModify for %s", key)
	var gcp *types.ConfigItemValueMap
	debug, gcp = agentlog.HandleGlobalConfig(ctx.subGlobalConfig, agentName,
		debugOverride)
	if gcp != nil {
		if gcp.GlobalValueInt(types.VdiskGCTime) != 0 {
			ctx.vdiskGCTime = gcp.GlobalValueInt(types.VdiskGCTime)
		}
		if gcp.GlobalValueInt(types.DownloadGCTime) != 0 {
			downloadGCTime = time.Duration(gcp.GlobalValueInt(types.DownloadGCTime)) * time.Second
		}
		ctx.globalConfig = gcp
		ctx.GCInitialized = true
	}
	log.Infof("handleGlobalConfigModify done for %s", key)
}

func handleGlobalConfigDelete(ctxArg interface{}, key string,
	statusArg interface{}) {

	ctx := ctxArg.(*volumemgrContext)
	if key != "global" {
		log.Infof("handleGlobalConfigDelete: ignoring %s", key)
		return
	}
	log.Infof("handleGlobalConfigDelete for %s", key)
	debug, _ = agentlog.HandleGlobalConfig(ctx.subGlobalConfig, agentName,
		debugOverride)
	*ctx.globalConfig = *types.DefaultConfigItemValueMap()
	log.Infof("handleGlobalConfigDelete done for %s", key)
}

func handleZedAgentStatusModify(ctxArg interface{}, key string,
	statusArg interface{}) {

	ctx := ctxArg.(*volumemgrContext)
	status := statusArg.(types.ZedAgentStatus)
	switch status.ConfigGetStatus {
	case types.ConfigGetSuccess, types.ConfigGetReadSaved:
		ctx.usingConfig = true
		duration := time.Duration(ctx.vdiskGCTime / 10)
		ctx.gc = time.NewTicker(duration * time.Second)
		gcResetPersistObjectLastUse(ctx)
	}
}

// for function name consistency
func handleAppImgModify(ctxArg interface{}, key string,
	configArg interface{}) {

	log.Infof("handleAppImgModify(%s)", key)
	config := configArg.(types.OldVolumeConfig)
	ctx := ctxArg.(*volumemgrContext)
	vcModify(ctx, types.AppImgObj, key, config)
}

func handleAppImgCreate(ctxArg interface{}, key string,
	configArg interface{}) {

	log.Infof("handleAppImgCreate(%s)", key)
	config := configArg.(types.OldVolumeConfig)
	ctx := ctxArg.(*volumemgrContext)
	vcCreate(ctx, types.AppImgObj, key, config)
}

func handleAppImgDelete(ctxArg interface{}, key string, configArg interface{}) {

	log.Infof("handleAppImageDelete(%s)", key)
	config := configArg.(types.OldVolumeConfig)
	ctx := ctxArg.(*volumemgrContext)
	vcDelete(ctx, types.AppImgObj, key, config)
}

func handleBaseOsModify(ctxArg interface{}, key string,
	configArg interface{}) {

	log.Infof("handleBaseOsModify(%s)", key)
	config := configArg.(types.OldVolumeConfig)
	ctx := ctxArg.(*volumemgrContext)
	vcModify(ctx, types.BaseOsObj, key, config)
}

func handleBaseOsCreate(ctxArg interface{}, key string,
	configArg interface{}) {

	log.Infof("handleBaseOsCreate(%s)", key)
	config := configArg.(types.OldVolumeConfig)
	ctx := ctxArg.(*volumemgrContext)
	vcCreate(ctx, types.BaseOsObj, key, config)
}

func handleBaseOsDelete(ctxArg interface{}, key string, configArg interface{}) {

	log.Infof("handleAppImageDelete(%s)", key)
	config := configArg.(types.OldVolumeConfig)
	ctx := ctxArg.(*volumemgrContext)
	vcDelete(ctx, types.BaseOsObj, key, config)
}

func handleCertObjConfigDelete(ctxArg interface{}, key string,
	configArg interface{}) {

	log.Infof("handleCertObjConfigDelete(%s)", key)
	ctx := ctxArg.(*volumemgrContext)
	status := lookupCertObjStatus(ctx, key)
	if status == nil {
		log.Infof("handleCertObjConfigDelete: unknown %s", key)
		return
	}
	handleCertObjDelete(ctx, key, status)
	log.Infof("handleCertObjConfigDelete(%s) done", key)
}

// certificate config/status event handlers
// certificate config create event
func handleCertObjCreate(ctxArg interface{}, key string, configArg interface{}) {
	ctx := ctxArg.(*volumemgrContext)
	config := configArg.(types.CertObjConfig)
	log.Infof("handleCertObjCreate for %s", key)

	status := types.CertObjStatus{
		UUIDandVersion: config.UUIDandVersion,
		ConfigSha256:   config.ConfigSha256,
	}

	status.StorageStatusList = make([]types.StorageStatus,
		len(config.StorageConfigList))

	for i, sc := range config.StorageConfigList {
		ss := &status.StorageStatusList[i]
		ss.Name = sc.Name
		ss.ImageID = sc.ImageID
		ss.FinalObjDir = types.CertificateDirname
	}

	publishCertObjStatus(ctx, &status)

	certObjHandleStatusUpdate(ctx, &config, &status)
}

// certificate config modify event
func handleCertObjModify(ctxArg interface{}, key string, configArg interface{}) {
	ctx := ctxArg.(*volumemgrContext)
	config := configArg.(types.CertObjConfig)
	status := lookupCertObjStatus(ctx, key)
	uuidStr := config.Key()
	log.Infof("handleCertObjModify for %s", uuidStr)

	if config.UUIDandVersion.Version != status.UUIDandVersion.Version {
		log.Infof("handleCertObjModify(%s), New config version %v", key,
			config.UUIDandVersion.Version)
		status.UUIDandVersion = config.UUIDandVersion
		publishCertObjStatus(ctx, status)

	}

	// on storage config change, purge and recreate
	if certObjCheckConfigModify(ctx, key, &config, status) {
		removeCertObjConfig(ctx, key)
		handleCertObjCreate(ctx, key, config)
	}
}

// certificate config delete event
func handleCertObjDelete(ctx *volumemgrContext, key string,
	status *types.CertObjStatus) {

	uuidStr := status.Key()
	log.Infof("handleCertObjDelete for %s", uuidStr)
	removeCertObjConfig(ctx, uuidStr)
}

// certs download status modify event
// Handles both create and modify events
func handleDownloadStatusModify(ctxArg interface{}, key string,
	statusArg interface{}) {

	status := statusArg.(types.DownloaderStatus)
	// XXX ctx := ctxArg.(*volumemgrContext)
	log.Infof("handleDownloadStatusModify for %s",
		status.ImageID)
	// XXX updateDownloaderStatus(ctx, &status)
	// XXX do we need to walk all certObj to find imageID, and call:
	// XXX certObjHandleStatusUpdate(ctx, &config, &status)
}

// certs download status delete event
func handleDownloadStatusDelete(ctxArg interface{}, key string,
	statusArg interface{}) {

	status := statusArg.(types.DownloaderStatus)
	log.Infof("handleDownloadStatusDelete RefCount %d Expired %v for %s",
		status.RefCount, status.Expired, key)
	// Nothing to do
}
