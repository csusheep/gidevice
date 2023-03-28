package giDevice

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/electricbubble/gidevice/pkg/libimobiledevice"
)

var _ Instruments = (*instruments)(nil)

var OutputCallBack func(m string)

func newInstruments(client *libimobiledevice.InstrumentsClient) *instruments {
	return &instruments{
		client: client,
	}
}

type instruments struct {
	client *libimobiledevice.InstrumentsClient
}

func (i *instruments) notifyOfPublishedCapabilities() (err error) {
	_, err = i.client.NotifyOfPublishedCapabilities()
	return
}

func (i *instruments) requestChannel(channel string) (id uint32, err error) {
	return i.client.RequestChannel(channel)
}

func (i *instruments) AppLaunch(bundleID string, opts ...AppLaunchOption) (pid int, err error) {
	opt := new(appLaunchOption)
	opt.appPath = ""
	opt.options = map[string]interface{}{
		"StartSuspendedKey": uint64(0),
		"KillExisting":      uint64(0),
	}
	if len(opts) != 0 {
		for _, optFunc := range opts {
			optFunc(opt)
		}
	}

	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.processcontrol"); err != nil {
		return 0, err
	}

	args := libimobiledevice.NewAuxBuffer()
	if err = args.AppendObject(opt.appPath); err != nil {
		return 0, err
	}
	if err = args.AppendObject(bundleID); err != nil {
		return 0, err
	}
	if err = args.AppendObject(opt.environment); err != nil {
		return 0, err
	}
	if err = args.AppendObject(opt.arguments); err != nil {
		return 0, err
	}
	if err = args.AppendObject(opt.options); err != nil {
		return 0, err
	}

	var result *libimobiledevice.DTXMessageResult
	selector := "launchSuspendedProcessWithDevicePath:bundleIdentifier:environment:arguments:options:"
	if result, err = i.client.Invoke(selector, args, id, true); err != nil {
		return 0, err
	}

	if nsErr, ok := result.Obj.(libimobiledevice.NSError); ok {
		return 0, fmt.Errorf("%s", nsErr.NSUserInfo.(map[string]interface{})["NSLocalizedDescription"])
	}

	return int(result.Obj.(uint64)), nil
}

func (i *instruments) appProcess(bundleID string) (err error) {
	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.processcontrol"); err != nil {
		return err
	}

	args := libimobiledevice.NewAuxBuffer()
	if err = args.AppendObject(bundleID); err != nil {
		return err
	}

	selector := "processIdentifierForBundleIdentifier:"
	if _, err = i.client.Invoke(selector, args, id, true); err != nil {
		return err
	}

	return
}

func (i *instruments) startObserving(pid int) (err error) {
	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.processcontrol"); err != nil {
		return err
	}

	args := libimobiledevice.NewAuxBuffer()
	if err = args.AppendObject(pid); err != nil {
		return err
	}

	var result *libimobiledevice.DTXMessageResult
	selector := "startObservingPid:"
	if result, err = i.client.Invoke(selector, args, id, true); err != nil {
		return err
	}

	if nsErr, ok := result.Obj.(libimobiledevice.NSError); ok {
		return fmt.Errorf("%s", nsErr.NSUserInfo.(map[string]interface{})["NSLocalizedDescription"])
	}
	return
}

func (i *instruments) AppKill(pid int) (err error) {
	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.processcontrol"); err != nil {
		return err
	}

	args := libimobiledevice.NewAuxBuffer()
	if err = args.AppendObject(pid); err != nil {
		return err
	}

	selector := "killPid:"
	if _, err = i.client.Invoke(selector, args, id, false); err != nil {
		return err
	}

	return
}

func (i *instruments) AppRunningProcesses() (processes []Process, err error) {
	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.deviceinfo"); err != nil {
		return nil, err
	}

	selector := "runningProcesses"

	var result *libimobiledevice.DTXMessageResult
	if result, err = i.client.Invoke(selector, libimobiledevice.NewAuxBuffer(), id, true); err != nil {
		return nil, err
	}

	objs := result.Obj.([]interface{})

	processes = make([]Process, 0, len(objs))

	for _, v := range objs {
		m := v.(map[string]interface{})

		var data []byte
		if data, err = json.Marshal(m); err != nil {
			debugLog(fmt.Sprintf("process marshal: %v\n%v\n", err, m))
			err = nil
			continue
		}

		var tp Process
		if err = json.Unmarshal(data, &tp); err != nil {
			debugLog(fmt.Sprintf("process unmarshal: %v\n%v\n", err, m))
			err = nil
			continue
		}

		processes = append(processes, tp)
	}

	return
}

func (i *instruments) AppList(opts ...AppListOption) (apps []Application, err error) {
	opt := new(appListOption)
	opt.updateToken = ""
	opt.appsMatching = make(map[string]interface{})
	if len(opts) != 0 {
		for _, optFunc := range opts {
			optFunc(opt)
		}
	}

	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.device.applictionListing"); err != nil {
		return nil, err
	}

	args := libimobiledevice.NewAuxBuffer()
	if err = args.AppendObject(opt.appsMatching); err != nil {
		return nil, err
	}
	if err = args.AppendObject(opt.updateToken); err != nil {
		return nil, err
	}

	selector := "installedApplicationsMatching:registerUpdateToken:"

	var result *libimobiledevice.DTXMessageResult
	if result, err = i.client.Invoke(selector, args, id, true); err != nil {
		return nil, err
	}

	objs := result.Obj.([]interface{})

	for _, v := range objs {
		m := v.(map[string]interface{})

		var data []byte
		if data, err = json.Marshal(m); err != nil {
			debugLog(fmt.Sprintf("application marshal: %v\n%v\n", err, m))
			err = nil
			continue
		}

		var app Application
		if err = json.Unmarshal(data, &app); err != nil {
			debugLog(fmt.Sprintf("application unmarshal: %v\n%v\n", err, m))
			err = nil
			continue
		}
		apps = append(apps, app)
	}

	return
}

func (i *instruments) DeviceInfo() (devInfo *DeviceInfo, err error) {
	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.deviceinfo"); err != nil {
		return nil, err
	}

	selector := "systemInformation"

	var result *libimobiledevice.DTXMessageResult
	if result, err = i.client.Invoke(selector, libimobiledevice.NewAuxBuffer(), id, true); err != nil {
		return nil, err
	}

	data, err := json.Marshal(result.Obj)
	if err != nil {
		return nil, err
	}
	devInfo = new(DeviceInfo)
	err = json.Unmarshal(data, devInfo)

	return
}

func (i *instruments) registerCallback(obj string, cb func(m libimobiledevice.DTXMessageResult)) {
	i.client.RegisterCallback(obj, cb)
}

type Application struct {
	AppExtensionUUIDs         []string `json:"AppExtensionUUIDs,omitempty"`
	BundlePath                string   `json:"BundlePath"`
	CFBundleIdentifier        string   `json:"CFBundleIdentifier"`
	ContainerBundleIdentifier string   `json:"ContainerBundleIdentifier,omitempty"`
	ContainerBundlePath       string   `json:"ContainerBundlePath,omitempty"`
	DisplayName               string   `json:"DisplayName"`
	ExecutableName            string   `json:"ExecutableName,omitempty"`
	Placeholder               bool     `json:"Placeholder,omitempty"`
	PluginIdentifier          string   `json:"PluginIdentifier,omitempty"`
	PluginUUID                string   `json:"PluginUUID,omitempty"`
	Restricted                int      `json:"Restricted"`
	Type                      string   `json:"Type"`
	Version                   string   `json:"Version"`
}

type DeviceInfo struct {
	Description       string `json:"_deviceDescription"`
	DisplayName       string `json:"_deviceDisplayName"`
	Identifier        string `json:"_deviceIdentifier"`
	Version           string `json:"_deviceVersion"`
	ProductType       string `json:"_productType"`
	ProductVersion    string `json:"_productVersion"`
	XRDeviceClassName string `json:"_xrdeviceClassName"`
}

func on_sysmontap_message(m libimobiledevice.DTXMessageResult) {
	data, err := json.Marshal(m.Obj)
	if err != nil {
		return
	}
	if OutputCallBack != nil {
		OutputCallBack(string(data))
	}
	fmt.Println(string(data))
}

func (i *instruments) Sysmontap(cb func(m string)) (err error) {
	OutputCallBack = cb
	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.sysmontap"); err != nil {
		return err
	}

	selector := "setConfig:"

	// var tmp AUSSysmontap
	// tmp.Ur = 1000
	// tmp.Bm = 0

	// tmp.ProcAttrs = []string{"memVirtualSize", "cpuUsage", "procStatus", "appSleep", "uid", "vmPageIns", "memRShrd", "ctxSwitch", "memCompressed", "intWakeups", "cpuTotalSystem", "responsiblePID", "physFootprint", "cpuTotalUser", "sysCallsUnix", "memResidentSize", "sysCallsMach", "memPurgeable", "diskBytesRead", "machPortCount", "__suddenTerm", "__arch", "memRPrvt", "msgSent", "ppid", "threadCount", "memAnon", "diskBytesWritten", "pgid", "faults", "msgRecv", "__restricted", "pid", "__sandbox"}
	// tmp.SysAttrs = []string{"diskWriteOps", "diskBytesRead", "diskBytesWritten", "threadCount", "vmCompressorPageCount", "vmExtPageCount", "vmFreeCount", "vmIntPageCount", "vmPurgeableCount", "netPacketsIn", "vmWireCount", "netBytesIn", "netPacketsOut", "diskReadOps", "vmUsedCount", "__vmSwapUsage", "netBytesOut"}
	// tmp.CPUUsage = true
	// tmp.SampleInterval = 1000000000
	// tmp = json.Marshal(tmp)
	tmp := make(map[string]interface{})
	tmp["ur"] = 1000
	tmp["bm"] = 0
	tmp["procAttrs"] = []string{"pid", "cpuUsage", "threadCount", "memVirtualSize", "vmPageIns", "memRShrd", "memCompressed"}
	tmp["sysAttrs"] = []string{"diskWriteOps", "diskBytesRead", "diskBytesWritten", "threadCount", "vmCompressorPageCount", "vmExtPageCount", "vmFreeCount", "vmIntPageCount", "vmPurgeableCount", "netPacketsIn", "vmWireCount", "netBytesIn", "netPacketsOut", "diskReadOps", "vmUsedCount", "__vmSwapUsage", "netBytesOut"}
	tmp["cpuUsage"] = true
	tmp["sampleInterval"] = time.Second * 1

	args := libimobiledevice.NewAuxBuffer()

	if err = args.AppendObject(tmp); err != nil {
		return err
	}

	var result *libimobiledevice.DTXMessageResult
	if result, err = i.client.Invoke(selector, args, id, true); err != nil {
		return err
	}

	data, err := json.Marshal(result.Obj)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	// devInfo = new(DeviceInfo)
	// err = json.Unmarshal(data, devInfo)

	i.registerCallback("", on_sysmontap_message)

	i.client.Invoke("start", libimobiledevice.NewAuxBuffer(), id, true)
	// time.Sleep(time.Second * 5)
	// i.client.Invoke("stop", libimobiledevice.NewAuxBuffer(), id, true)
	return
}

func (i *instruments) StopSysmontap() (err error) {
	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.sysmontap"); err != nil {
		return err
	}
	i.client.Invoke("stop", libimobiledevice.NewAuxBuffer(), id, true)
	OutputCallBack = nil
	return
}
