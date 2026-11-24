package backend

import (
	"fmt"
	"runtime"
	"strings"

	"ant-chrome/backend/internal/browser"
)

type BrowserTemplateCapabilityCheck struct {
	Capability string `json:"capability"`
	Status     string `json:"status"`
	Verified   bool   `json:"verified"`
	Note       string `json:"note,omitempty"`
}

type BrowserTemplateCompatibility struct {
	SchemaVersion int                              `json:"schemaVersion"`
	Bound         bool                             `json:"bound"`
	TemplateID    string                           `json:"templateId,omitempty"`
	Revision      string                           `json:"revision,omitempty"`
	TargetOS      string                           `json:"targetOS,omitempty"`
	DeviceClass   string                           `json:"deviceClass,omitempty"`
	KernelFamily  string                           `json:"kernelFamily,omitempty"`
	SupportLevel  string                           `json:"supportLevel,omitempty"`
	Eligible      bool                             `json:"eligible"`
	Verified      bool                             `json:"verified"`
	Status        string                           `json:"status"`
	Reasons       []string                         `json:"reasons,omitempty"`
	Requirements  []BrowserTemplateCapabilityCheck `json:"requirements,omitempty"`
}

func normalizedTemplateHostOS() string {
	switch runtime.GOOS {
	case "darwin": return "macos"
	default: return strings.ToLower(runtime.GOOS)
	}
}

func findCapabilityStatus(manifest BrowserKernelCapabilityManifest, name string) (BrowserKernelCapabilityStatus,bool) {
	name=strings.TrimSpace(name)
	for _,item:=range manifest.Capabilities{
		if strings.EqualFold(strings.TrimSpace(item.Capability),name){return item,true}
	}
	return BrowserKernelCapabilityStatus{},false
}

func (a *App) BrowserProfileTemplateCompatibility(profileID string) (*BrowserTemplateCompatibility,error) {
	if a==nil||a.browserMgr==nil{return nil,fmt.Errorf("browser manager is unavailable")}
	contract,err:=a.browserMgr.EnvironmentContract(profileID);if err!=nil{return nil,err}
	out:=&BrowserTemplateCompatibility{SchemaVersion:1,Status:"unbound",Eligible:true}
	if contract.Template==nil{return out,nil}
	out.Bound=true;out.TemplateID=contract.Template.TemplateID;out.Revision=contract.Template.TemplateRevision
	template,ok:=browser.FindEnvironmentTemplate(contract.Template.TemplateID)
	if !ok{
		out.Eligible=false;out.Status="template-missing";out.Reasons=[]string{"bound template definition is unavailable"}
		return out,nil
	}
	out.TargetOS=template.TargetOS;out.DeviceClass=template.DeviceClass;out.KernelFamily=template.KernelFamily;out.SupportLevel=template.SupportLevel
	hostOS:=normalizedTemplateHostOS()
	if template.TargetOS!=""&&!strings.EqualFold(template.TargetOS,hostOS){
		out.Eligible=false;out.Status="incompatible-host"
		out.Reasons=append(out.Reasons,fmt.Sprintf("template targets %s but current runtime host is %s",template.TargetOS,hostOS))
	}
	manifest,manifestErr:=a.BrowserProfileKernelCapabilityManifest(profileID)
	if manifestErr!=nil{
		out.Eligible=false;out.Status="kernel-unavailable";out.Reasons=append(out.Reasons,manifestErr.Error());return out,nil
	}
	if strings.TrimSpace(manifest.CoreID)==""||strings.TrimSpace(manifest.BrowserVersion)==""{
		out.Eligible=false;out.Status="kernel-unavailable";out.Reasons=append(out.Reasons,"no usable kernel identity/version is available")
	}
	allVerified:=len(template.RequiredCapabilities)>0
	for _,required:=range template.RequiredCapabilities{
		check:=BrowserTemplateCapabilityCheck{Capability:required,Status:"unmapped",Note:"required capability is declared by the compatibility pack but has no exact manifest evidence key"}
		if actual,found:=findCapabilityStatus(manifest,required);found{
			check.Status=actual.Status;check.Verified=actual.Verified;check.Note=actual.Note
		}
		if !check.Verified{allVerified=false}
		out.Requirements=append(out.Requirements,check)
	}
	out.Verified=out.Eligible&&allVerified
	if !out.Eligible{
		if out.Status==""||out.Status=="unbound"{out.Status="incompatible"}
	}else if out.Verified{
		out.Status="verified"
	}else{
		out.Status="requires-verification"
		out.Reasons=append(out.Reasons,"declared template requirements are not all backed by verified kernel/runtime evidence")
	}
	return out,nil
}
