// Copyright (c) 2018 Zededa, Inc.
// All rights reserved.

package cast

import (
	"encoding/json"
	"github.com/zededa/go-provision/types"
	"log"
)

// XXX template?
// XXX alternative seems to be a deep copy of some sort

func CastNetworkObjectConfig(in interface{}) types.NetworkObjectConfig {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastNetworkObjectConfig")
	}
	var output types.NetworkObjectConfig
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastNetworkObjectConfig")
	}
	return output
}

func CastNetworkObjectStatus(in interface{}) types.NetworkObjectStatus {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastNetworkObjectStatus")
	}
	var output types.NetworkObjectStatus
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastNetworkObjectStatus")
	}
	return output
}

func CastNetworkServiceConfig(in interface{}) types.NetworkServiceConfig {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastNetworkServiceConfig")
	}
	var output types.NetworkServiceConfig
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastNetworkServiceConfig")
	}
	return output
}

func CastNetworkServiceStatus(in interface{}) types.NetworkServiceStatus {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastNetworkServiceStatus")
	}
	var output types.NetworkServiceStatus
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastNetworkServiceStatus")
	}
	return output
}

func CastDeviceNetworkStatus(in interface{}) types.DeviceNetworkStatus {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastDeviceNetworkStatus")
	}
	var output types.DeviceNetworkStatus
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastDeviceNetworkStatus")
	}
	return output
}

func CastAppInstanceConfig(in interface{}) types.AppInstanceConfig {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastAppInstanceConfig")
	}
	var output types.AppInstanceConfig
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastAppInstanceConfig")
	}
	return output
}

func CastAppInstanceStatus(in interface{}) types.AppInstanceStatus {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastAppInstanceStatus")
	}
	var output types.AppInstanceStatus
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastAppInstanceStatus")
	}
	return output
}

func CastAppNetworkConfig(in interface{}) types.AppNetworkConfig {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastAppNetworkConfig")
	}
	var output types.AppNetworkConfig
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastAppNetworkConfig")
	}
	return output
}

func CastAppNetworkStatus(in interface{}) types.AppNetworkStatus {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastAppNetworkStatus")
	}
	var output types.AppNetworkStatus
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastAppNetworkStatus")
	}
	return output
}

func CastDomainConfig(in interface{}) types.DomainConfig {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastDomainConfig")
	}
	var output types.DomainConfig
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastDomainConfig")
	}
	return output
}

func CastDomainStatus(in interface{}) types.DomainStatus {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastDomainStatus")
	}
	var output types.DomainStatus
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastDomainStatus")
	}
	return output
}

func CastEIDConfig(in interface{}) types.EIDConfig {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastEIDConfig")
	}
	var output types.EIDConfig
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastEIDConfig")
	}
	return output
}

func CastEIDStatus(in interface{}) types.EIDStatus {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastEIDStatus")
	}
	var output types.EIDStatus
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastEIDStatus")
	}
	return output
}

func CastCertObjConfig(in interface{}) types.CertObjConfig {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastCertObjConfig")
	}
	var output types.CertObjConfig
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastCertObjConfig")
	}
	return output
}

func CastCertObjStatus(in interface{}) types.CertObjStatus {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastCertObjStatus")
	}
	var output types.CertObjStatus
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastCertObjStatus")
	}
	return output
}

func CastBaseOsConfig(in interface{}) types.BaseOsConfig {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastBaseOsConfig")
	}
	var output types.BaseOsConfig
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastBaseOsConfig")
	}
	return output
}

func CastBaseOsStatus(in interface{}) types.BaseOsStatus {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastBaseOsStatus")
	}
	var output types.BaseOsStatus
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastBaseOsStatus")
	}
	return output
}

func CastDownloaderConfig(in interface{}) types.DownloaderConfig {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastDownloaderConfig")
	}
	var output types.DownloaderConfig
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastDownloaderConfig")
	}
	return output
}

func CastDownloaderStatus(in interface{}) types.DownloaderStatus {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastDownloaderStatus")
	}
	var output types.DownloaderStatus
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastDownloaderStatus")
	}
	return output
}

func CastVerifyImageConfig(in interface{}) types.VerifyImageConfig {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastVerifyImageConfig")
	}
	var output types.VerifyImageConfig
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastVerifyImageConfig")
	}
	return output
}

func CastVerifyImageStatus(in interface{}) types.VerifyImageStatus {
	b, err := json.Marshal(in)
	if err != nil {
		log.Fatal(err, "json Marshal in CastVerifyImageStatus")
	}
	var output types.VerifyImageStatus
	if err := json.Unmarshal(b, &output); err != nil {
		log.Fatal(err, "json Unmarshal in CastVerifyImageStatus")
	}
	return output
}
