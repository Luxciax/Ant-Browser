export namespace automation {
	
	export class ScriptPublicAPIVariable {
	    name: string;
	    defaultValue: string;
	    description: string;
	    required: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScriptPublicAPIVariable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.defaultValue = source["defaultValue"];
	        this.description = source["description"];
	        this.required = source["required"];
	    }
	}
	export class ScriptPublicAPIConfig {
	    enabled: boolean;
	    method: string;
	    path: string;
	    requestMode: string;
	    responseMode: string;
	    timeoutMs: number;
	    requestBodyText: string;
	    responseBodyText: string;
	    variables: ScriptPublicAPIVariable[];
	
	    static createFrom(source: any = {}) {
	        return new ScriptPublicAPIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.requestMode = source["requestMode"];
	        this.responseMode = source["responseMode"];
	        this.timeoutMs = source["timeoutMs"];
	        this.requestBodyText = source["requestBodyText"];
	        this.responseBodyText = source["responseBodyText"];
	        this.variables = this.convertValues(source["variables"], ScriptPublicAPIVariable);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ScriptSource {
	    type: string;
	    uri: string;
	    ref: string;
	    path: string;
	    importedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.uri = source["uri"];
	        this.ref = source["ref"];
	        this.path = source["path"];
	        this.importedAt = source["importedAt"];
	    }
	}
	export class ScriptTargetSelector {
	    code: string;
	    profileId: string;
	    profileName: string;
	    groupId: string;
	    keywords: string[];
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new ScriptTargetSelector(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.groupId = source["groupId"];
	        this.keywords = source["keywords"];
	        this.tags = source["tags"];
	    }
	}
	export class ScriptTargetConfig {
	    mode: string;
	    selector: ScriptTargetSelector;
	    templateSelector: ScriptTargetSelector;
	    createNameTemplate: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptTargetConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.selector = this.convertValues(source["selector"], ScriptTargetSelector);
	        this.templateSelector = this.convertValues(source["templateSelector"], ScriptTargetSelector);
	        this.createNameTemplate = source["createNameTemplate"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ScriptRecord {
	    packageFormat: string;
	    manifestVersion: number;
	    id: string;
	    name: string;
	    description: string;
	    type: string;
	    status: string;
	    entryFile: string;
	    tags: string[];
	    selectorText: string;
	    paramsText: string;
	    scriptText: string;
	    notes: string;
	    targetConfig: ScriptTargetConfig;
	    publicAPI: ScriptPublicAPIConfig;
	    source: ScriptSource;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ScriptRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packageFormat = source["packageFormat"];
	        this.manifestVersion = source["manifestVersion"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.entryFile = source["entryFile"];
	        this.tags = source["tags"];
	        this.selectorText = source["selectorText"];
	        this.paramsText = source["paramsText"];
	        this.scriptText = source["scriptText"];
	        this.notes = source["notes"];
	        this.targetConfig = this.convertValues(source["targetConfig"], ScriptTargetConfig);
	        this.publicAPI = this.convertValues(source["publicAPI"], ScriptPublicAPIConfig);
	        this.source = this.convertValues(source["source"], ScriptSource);
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ScriptRunRecord {
	    id: string;
	    scriptId: string;
	    scriptName: string;
	    scriptType: string;
	    status: string;
	    summary: string;
	    error: string;
	    resultText: string;
	    logText: string;
	    startedAt: string;
	    finishedAt: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new ScriptRunRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.scriptId = source["scriptId"];
	        this.scriptName = source["scriptName"];
	        this.scriptType = source["scriptType"];
	        this.status = source["status"];
	        this.summary = source["summary"];
	        this.error = source["error"];
	        this.resultText = source["resultText"];
	        this.logText = source["logText"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.durationMs = source["durationMs"];
	    }
	}
	export class ScriptRunRequest {
	    scriptId: string;
	    selectorText: string;
	    targetMode?: string;
	    targetInput?: any;
	    paramsText: string;
	    useScriptSelector: boolean;
	    useScriptParams: boolean;
	    timeoutMs?: number;
	
	    static createFrom(source: any = {}) {
	        return new ScriptRunRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scriptId = source["scriptId"];
	        this.selectorText = source["selectorText"];
	        this.targetMode = source["targetMode"];
	        this.targetInput = source["targetInput"];
	        this.paramsText = source["paramsText"];
	        this.useScriptSelector = source["useScriptSelector"];
	        this.useScriptParams = source["useScriptParams"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}
	
	
	
	export class WorkflowCondition {
	    variable: string;
	    operator: string;
	    value?: any;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowCondition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variable = source["variable"];
	        this.operator = source["operator"];
	        this.value = source["value"];
	    }
	}
	export class WorkflowSchedule {
	    enabled: boolean;
	    everyMinutes?: number;
	    dailyTime?: string;
	    allowOverlap?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowSchedule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.everyMinutes = source["everyMinutes"];
	        this.dailyTime = source["dailyTime"];
	        this.allowOverlap = source["allowOverlap"];
	    }
	}
	export class WorkflowRetryPolicy {
	    maxAttempts?: number;
	    delayMs?: number;
	    allowSideEffects?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowRetryPolicy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxAttempts = source["maxAttempts"];
	        this.delayMs = source["delayMs"];
	        this.allowSideEffects = source["allowSideEffects"];
	    }
	}
	export class WorkflowStep {
	    id: string;
	    type: string;
	    action?: string;
	    args?: Record<string, any>;
	    saveAs?: string;
	    value?: any;
	    condition?: WorkflowCondition;
	    steps?: WorkflowStep[];
	    elseSteps?: WorkflowStep[];
	    maxIterations?: number;
	    retry?: WorkflowRetryPolicy;
	    timeoutMs?: number;
	    breakpoint?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.action = source["action"];
	        this.args = source["args"];
	        this.saveAs = source["saveAs"];
	        this.value = source["value"];
	        this.condition = this.convertValues(source["condition"], WorkflowCondition);
	        this.steps = this.convertValues(source["steps"], WorkflowStep);
	        this.elseSteps = this.convertValues(source["elseSteps"], WorkflowStep);
	        this.maxIterations = source["maxIterations"];
	        this.retry = this.convertValues(source["retry"], WorkflowRetryPolicy);
	        this.timeoutMs = source["timeoutMs"];
	        this.breakpoint = source["breakpoint"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WorkflowSelector {
	    code?: string;
	    profileId?: string;
	    profileName?: string;
	    groupId?: string;
	    keywords?: string[];
	    tags?: string[];
	    matchMode?: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowSelector(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.groupId = source["groupId"];
	        this.keywords = source["keywords"];
	        this.tags = source["tags"];
	        this.matchMode = source["matchMode"];
	    }
	}
	export class WorkflowDefinition {
	    schemaVersion: number;
	    id: string;
	    name: string;
	    description?: string;
	    enabled: boolean;
	    selector: WorkflowSelector;
	    variables?: Record<string, any>;
	    steps: WorkflowStep[];
	    schedule: WorkflowSchedule;
	    createdAt: string;
	    updatedAt: string;
	    lastRunAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowDefinition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.enabled = source["enabled"];
	        this.selector = this.convertValues(source["selector"], WorkflowSelector);
	        this.variables = source["variables"];
	        this.steps = this.convertValues(source["steps"], WorkflowStep);
	        this.schedule = this.convertValues(source["schedule"], WorkflowSchedule);
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastRunAt = source["lastRunAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class WorkflowRunStepRecord {
	    stepId: string;
	    stepType: string;
	    action?: string;
	    index: number;
	    attempt: number;
	    ok: boolean;
	    result?: Record<string, any>;
	    error?: string;
	    startedAt: string;
	    finishedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowRunStepRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stepId = source["stepId"];
	        this.stepType = source["stepType"];
	        this.action = source["action"];
	        this.index = source["index"];
	        this.attempt = source["attempt"];
	        this.ok = source["ok"];
	        this.result = source["result"];
	        this.error = source["error"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	    }
	}
	export class WorkflowRunRecord {
	    schemaVersion: number;
	    id: string;
	    workflowId: string;
	    workflowName: string;
	    status: string;
	    selector: WorkflowSelector;
	    variables: Record<string, any>;
	    nextStep: number;
	    lastStepId?: string;
	    resumeRequiresForce?: boolean;
	    steps: WorkflowRunStepRecord[];
	    error?: string;
	    startedAt: string;
	    finishedAt?: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowRunRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.workflowId = source["workflowId"];
	        this.workflowName = source["workflowName"];
	        this.status = source["status"];
	        this.selector = this.convertValues(source["selector"], WorkflowSelector);
	        this.variables = source["variables"];
	        this.nextStep = source["nextStep"];
	        this.lastStepId = source["lastStepId"];
	        this.resumeRequiresForce = source["resumeRequiresForce"];
	        this.steps = this.convertValues(source["steps"], WorkflowRunStepRecord);
	        this.error = source["error"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WorkflowRunRequest {
	    workflowId: string;
	    variables?: Record<string, any>;
	    breakpoints?: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorkflowRunRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workflowId = source["workflowId"];
	        this.variables = source["variables"];
	        this.breakpoints = source["breakpoints"];
	    }
	}
	
	
	

}

export namespace backend {
	
	export class AutomationScriptImportIssue {
	    path: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new AutomationScriptImportIssue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.message = source["message"];
	    }
	}
	export class AutomationScriptBatchImportResult {
	    imported: automation.ScriptRecord[];
	    failed: AutomationScriptImportIssue[];
	    scanned: number;
	
	    static createFrom(source: any = {}) {
	        return new AutomationScriptBatchImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = this.convertValues(source["imported"], automation.ScriptRecord);
	        this.failed = this.convertValues(source["failed"], AutomationScriptImportIssue);
	        this.scanned = source["scanned"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class AutomationScriptPublicAPIInvokeInput {
	    url: string;
	    method: string;
	    bodyText: string;
	    apiKey: string;
	    authHeader: string;
	    timeoutMs: number;
	
	    static createFrom(source: any = {}) {
	        return new AutomationScriptPublicAPIInvokeInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.method = source["method"];
	        this.bodyText = source["bodyText"];
	        this.apiKey = source["apiKey"];
	        this.authHeader = source["authHeader"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}
	export class AutomationScriptPublicAPIInvokeResult {
	    ok: boolean;
	    status: number;
	    statusText: string;
	    bodyText: string;
	    bodyJson: any;
	
	    static createFrom(source: any = {}) {
	        return new AutomationScriptPublicAPIInvokeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.statusText = source["statusText"];
	        this.bodyText = source["bodyText"];
	        this.bodyJson = source["bodyJson"];
	    }
	}
	export class BackupLocalHistoryItem {
	    name: string;
	    path: string;
	    size: number;
	    modifiedAt: string;
	    createdAt?: string;
	    metadataAvailable: boolean;
	    metadataError?: string;
	    metadataOrphan?: boolean;
	    appName?: string;
	    appVersion?: string;
	    packageType?: string;
	    profileCount?: number;
	    profileNames?: string[];
	
	    static createFrom(source: any = {}) {
	        return new BackupLocalHistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.modifiedAt = source["modifiedAt"];
	        this.createdAt = source["createdAt"];
	        this.metadataAvailable = source["metadataAvailable"];
	        this.metadataError = source["metadataError"];
	        this.metadataOrphan = source["metadataOrphan"];
	        this.appName = source["appName"];
	        this.appVersion = source["appVersion"];
	        this.packageType = source["packageType"];
	        this.profileCount = source["profileCount"];
	        this.profileNames = source["profileNames"];
	    }
	}
	export class BackupLocalSettings {
	    localDirectory: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupLocalSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.localDirectory = source["localDirectory"];
	    }
	}
	export class BackupSelectLocalDirectoryResult {
	    cancelled: boolean;
	    localDirectory: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupSelectLocalDirectoryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cancelled = source["cancelled"];
	        this.localDirectory = source["localDirectory"];
	    }
	}
	export class BookmarkSyncResult {
	    total: number;
	    synced: number;
	    skipped: number;
	    failed: number;
	    skippedList: string[];
	    failedList: string[];
	
	    static createFrom(source: any = {}) {
	        return new BookmarkSyncResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.synced = source["synced"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.skippedList = source["skippedList"];
	        this.failedList = source["failedList"];
	    }
	}
	export class BrowserBatchCreateItem {
	    clientKey?: string;
	    config: browser.ProfileInput;
	
	    static createFrom(source: any = {}) {
	        return new BrowserBatchCreateItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientKey = source["clientKey"];
	        this.config = this.convertValues(source["config"], browser.ProfileInput);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserBatchCreateRequest {
	    items: BrowserBatchCreateItem[];
	    concurrency?: number;
	
	    static createFrom(source: any = {}) {
	        return new BrowserBatchCreateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], BrowserBatchCreateItem);
	        this.concurrency = source["concurrency"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserBatchItemResult {
	    index: number;
	    clientKey?: string;
	    profileId?: string;
	    profileName?: string;
	    completed: boolean;
	    ok: boolean;
	    error?: string;
	    runtime?: browser.Profile;
	
	    static createFrom(source: any = {}) {
	        return new BrowserBatchItemResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.clientKey = source["clientKey"];
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.completed = source["completed"];
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.runtime = this.convertValues(source["runtime"], browser.Profile);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserBatchOperation {
	    operationId: string;
	    kind: string;
	    status: string;
	    concurrency: number;
	    total: number;
	    succeeded: number;
	    failed: number;
	    startedAt: string;
	    finishedAt?: string;
	    results: BrowserBatchItemResult[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserBatchOperation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.operationId = source["operationId"];
	        this.kind = source["kind"];
	        this.status = source["status"];
	        this.concurrency = source["concurrency"];
	        this.total = source["total"];
	        this.succeeded = source["succeeded"];
	        this.failed = source["failed"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.results = this.convertValues(source["results"], BrowserBatchItemResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserBatchProfilesRequest {
	    profileIds: string[];
	    concurrency?: number;
	
	    static createFrom(source: any = {}) {
	        return new BrowserBatchProfilesRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileIds = source["profileIds"];
	        this.concurrency = source["concurrency"];
	    }
	}
	export class BrowserTemplateCapabilityCheck {
	    capability: string;
	    status: string;
	    verified: boolean;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserTemplateCapabilityCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.capability = source["capability"];
	        this.status = source["status"];
	        this.verified = source["verified"];
	        this.note = source["note"];
	    }
	}
	export class BrowserTemplateCompatibility {
	    schemaVersion: number;
	    bound: boolean;
	    templateId?: string;
	    revision?: string;
	    targetOS?: string;
	    deviceClass?: string;
	    kernelFamily?: string;
	    supportLevel?: string;
	    eligible: boolean;
	    verified: boolean;
	    status: string;
	    reasons?: string[];
	    requirements?: BrowserTemplateCapabilityCheck[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserTemplateCompatibility(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.bound = source["bound"];
	        this.templateId = source["templateId"];
	        this.revision = source["revision"];
	        this.targetOS = source["targetOS"];
	        this.deviceClass = source["deviceClass"];
	        this.kernelFamily = source["kernelFamily"];
	        this.supportLevel = source["supportLevel"];
	        this.eligible = source["eligible"];
	        this.verified = source["verified"];
	        this.status = source["status"];
	        this.reasons = source["reasons"];
	        this.requirements = this.convertValues(source["requirements"], BrowserTemplateCapabilityCheck);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserEnvironmentViewV2 {
	    schemaVersion: number;
	    contract: Record<string, any>;
	    launchState: Record<string, any>;
	    launchReceipt?: Record<string, any>;
	    capabilities: Record<string, any>;
	    runtimeEvidence?: Record<string, any>;
	    templates: browser.EnvironmentTemplate[];
	    templateCompatibility?: BrowserTemplateCompatibility;
	
	    static createFrom(source: any = {}) {
	        return new BrowserEnvironmentViewV2(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.contract = source["contract"];
	        this.launchState = source["launchState"];
	        this.launchReceipt = source["launchReceipt"];
	        this.capabilities = source["capabilities"];
	        this.runtimeEvidence = source["runtimeEvidence"];
	        this.templates = this.convertValues(source["templates"], browser.EnvironmentTemplate);
	        this.templateCompatibility = this.convertValues(source["templateCompatibility"], BrowserTemplateCompatibility);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserExtensionManualDownloadFile {
	    fileName: string;
	    filePath: string;
	    sizeBytes: number;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserExtensionManualDownloadFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.filePath = source["filePath"];
	        this.sizeBytes = source["sizeBytes"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class BrowserExtensionManualInstallGuide {
	    extensionId: string;
	    storeUrl: string;
	    downloadUrl: string;
	    downloadDir: string;
	    fileName: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserExtensionManualInstallGuide(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extensionId = source["extensionId"];
	        this.storeUrl = source["storeUrl"];
	        this.downloadUrl = source["downloadUrl"];
	        this.downloadDir = source["downloadDir"];
	        this.fileName = source["fileName"];
	    }
	}
	export class BrowserExtensionWebStoreRequest {
	    query: string;
	    useProxy: boolean;
	    proxyConfig: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserExtensionWebStoreRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.useProxy = source["useProxy"];
	        this.proxyConfig = source["proxyConfig"];
	    }
	}
	export class BrowserFingerprintCapabilityRow {
	    capability: string;
	    status: string;
	    inputArg: string;
	    runtimeArg: string;
	    action: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintCapabilityRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.capability = source["capability"];
	        this.status = source["status"];
	        this.inputArg = source["inputArg"];
	        this.runtimeArg = source["runtimeArg"];
	        this.action = source["action"];
	        this.note = source["note"];
	    }
	}
	export class BrowserFingerprintCapabilityReport {
	    profileId: string;
	    coreId: string;
	    coreName: string;
	    chromeVersion: string;
	    chromeMajor: number;
	    versionStatus: string;
	    rawArgs: string[];
	    launchArgs: string[];
	    rows: BrowserFingerprintCapabilityRow[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintCapabilityReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.coreId = source["coreId"];
	        this.coreName = source["coreName"];
	        this.chromeVersion = source["chromeVersion"];
	        this.chromeMajor = source["chromeMajor"];
	        this.versionStatus = source["versionStatus"];
	        this.rawArgs = source["rawArgs"];
	        this.launchArgs = source["launchArgs"];
	        this.rows = this.convertValues(source["rows"], BrowserFingerprintCapabilityRow);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class BrowserFingerprintExpectedInfo {
	    language: string;
	    acceptLanguage: string;
	    timezone: string;
	    hardwareConcurrency: string;
	    deviceMemory: string;
	    colorDepth: string;
	    touchPoints: string;
	    windowSize: string;
	    brand: string;
	    brandVersion: string;
	    platform: string;
	    platformVersion: string;
	    seed: string;
	    disableSpoofing: string;
	    webrtcPolicy: string;
	    doNotTrack: string;
	    mediaDevices: string;
	    canvasNoise: string;
	    audioNoise: string;
	    clientRectsNoise: string;
	    fontList: string;
	    webglVendor: string;
	    webglRenderer: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintExpectedInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.acceptLanguage = source["acceptLanguage"];
	        this.timezone = source["timezone"];
	        this.hardwareConcurrency = source["hardwareConcurrency"];
	        this.deviceMemory = source["deviceMemory"];
	        this.colorDepth = source["colorDepth"];
	        this.touchPoints = source["touchPoints"];
	        this.windowSize = source["windowSize"];
	        this.brand = source["brand"];
	        this.brandVersion = source["brandVersion"];
	        this.platform = source["platform"];
	        this.platformVersion = source["platformVersion"];
	        this.seed = source["seed"];
	        this.disableSpoofing = source["disableSpoofing"];
	        this.webrtcPolicy = source["webrtcPolicy"];
	        this.doNotTrack = source["doNotTrack"];
	        this.mediaDevices = source["mediaDevices"];
	        this.canvasNoise = source["canvasNoise"];
	        this.audioNoise = source["audioNoise"];
	        this.clientRectsNoise = source["clientRectsNoise"];
	        this.fontList = source["fontList"];
	        this.webglVendor = source["webglVendor"];
	        this.webglRenderer = source["webglRenderer"];
	    }
	}
	export class BrowserFingerprintContextStatus {
	    applicable: boolean;
	    executed: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintContextStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.applicable = source["applicable"];
	        this.executed = source["executed"];
	        this.error = source["error"];
	    }
	}
	export class BrowserFingerprintWorkerInfo {
	    applicable: boolean;
	    executed: boolean;
	    language: string;
	    languages: string[];
	    timezone: string;
	    hardwareConcurrency: number;
	    platform: string;
	    userAgent: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintWorkerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.applicable = source["applicable"];
	        this.executed = source["executed"];
	        this.language = source["language"];
	        this.languages = source["languages"];
	        this.timezone = source["timezone"];
	        this.hardwareConcurrency = source["hardwareConcurrency"];
	        this.platform = source["platform"];
	        this.userAgent = source["userAgent"];
	        this.error = source["error"];
	    }
	}
	export class BrowserFingerprintRuntimeInfo {
	    worker?: BrowserFingerprintWorkerInfo;
	    iframe?: BrowserFingerprintWorkerInfo;
	    sharedWorker?: BrowserFingerprintWorkerInfo;
	    serviceWorker: BrowserFingerprintContextStatus;
	    language: string;
	    languages: string[];
	    timezone: string;
	    hardwareConcurrency: number;
	    deviceMemory: number;
	    maxTouchPoints: number;
	    doNotTrack: string;
	    mediaDeviceCount: number;
	    platform: string;
	    userAgent: string;
	    userAgentData: string;
	    webdriver: boolean;
	    screenWidth: number;
	    screenHeight: number;
	    colorDepth: number;
	    innerWidth: number;
	    innerHeight: number;
	    outerWidth: number;
	    outerHeight: number;
	    devicePixelRatio: number;
	    webglVendor: string;
	    webglRenderer: string;
	    canvasHash: string;
	    audioHash: string;
	    clientRectsHash: string;
	    plugins: string[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintRuntimeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.worker = this.convertValues(source["worker"], BrowserFingerprintWorkerInfo);
	        this.iframe = this.convertValues(source["iframe"], BrowserFingerprintWorkerInfo);
	        this.sharedWorker = this.convertValues(source["sharedWorker"], BrowserFingerprintWorkerInfo);
	        this.serviceWorker = this.convertValues(source["serviceWorker"], BrowserFingerprintContextStatus);
	        this.language = source["language"];
	        this.languages = source["languages"];
	        this.timezone = source["timezone"];
	        this.hardwareConcurrency = source["hardwareConcurrency"];
	        this.deviceMemory = source["deviceMemory"];
	        this.maxTouchPoints = source["maxTouchPoints"];
	        this.doNotTrack = source["doNotTrack"];
	        this.mediaDeviceCount = source["mediaDeviceCount"];
	        this.platform = source["platform"];
	        this.userAgent = source["userAgent"];
	        this.userAgentData = source["userAgentData"];
	        this.webdriver = source["webdriver"];
	        this.screenWidth = source["screenWidth"];
	        this.screenHeight = source["screenHeight"];
	        this.colorDepth = source["colorDepth"];
	        this.innerWidth = source["innerWidth"];
	        this.innerHeight = source["innerHeight"];
	        this.outerWidth = source["outerWidth"];
	        this.outerHeight = source["outerHeight"];
	        this.devicePixelRatio = source["devicePixelRatio"];
	        this.webglVendor = source["webglVendor"];
	        this.webglRenderer = source["webglRenderer"];
	        this.canvasHash = source["canvasHash"];
	        this.audioHash = source["audioHash"];
	        this.clientRectsHash = source["clientRectsHash"];
	        this.plugins = source["plugins"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserFingerprintCheckResult {
	    profileId: string;
	    status: string;
	    issues: string[];
	    limitations: string[];
	    runtime: BrowserFingerprintRuntimeInfo;
	    expected: BrowserFingerprintExpectedInfo;
	
	    static createFrom(source: any = {}) {
	        return new BrowserFingerprintCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.status = source["status"];
	        this.issues = source["issues"];
	        this.limitations = source["limitations"];
	        this.runtime = this.convertValues(source["runtime"], BrowserFingerprintRuntimeInfo);
	        this.expected = this.convertValues(source["expected"], BrowserFingerprintExpectedInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class BrowserKernelCapabilityStatus {
	    capability: string;
	    configurable: boolean;
	    emitted: boolean;
	    acknowledged: boolean;
	    observed: boolean;
	    verified: boolean;
	    status: string;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserKernelCapabilityStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.capability = source["capability"];
	        this.configurable = source["configurable"];
	        this.emitted = source["emitted"];
	        this.acknowledged = source["acknowledged"];
	        this.observed = source["observed"];
	        this.verified = source["verified"];
	        this.status = source["status"];
	        this.note = source["note"];
	    }
	}
	export class BrowserKernelCapabilityManifest {
	    schemaVersion: number;
	    coreId: string;
	    coreName: string;
	    browserVersion: string;
	    binarySha256?: string;
	    os: string;
	    arch: string;
	    configChannel: string;
	    generatedAt: string;
	    capabilities: BrowserKernelCapabilityStatus[];
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserKernelCapabilityManifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.coreId = source["coreId"];
	        this.coreName = source["coreName"];
	        this.browserVersion = source["browserVersion"];
	        this.binarySha256 = source["binarySha256"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.configChannel = source["configChannel"];
	        this.generatedAt = source["generatedAt"];
	        this.capabilities = this.convertValues(source["capabilities"], BrowserKernelCapabilityStatus);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class BrowserKernelUpgradePreview {
	    profileId: string;
	    currentCoreId?: string;
	    targetCoreId: string;
	    current: BrowserKernelCapabilityManifest;
	    target: BrowserKernelCapabilityManifest;
	    requiresRestart: boolean;
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserKernelUpgradePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.currentCoreId = source["currentCoreId"];
	        this.targetCoreId = source["targetCoreId"];
	        this.current = this.convertValues(source["current"], BrowserKernelCapabilityManifest);
	        this.target = this.convertValues(source["target"], BrowserKernelCapabilityManifest);
	        this.requiresRestart = source["requiresRestart"];
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserLaunchArgumentSummary {
	    profileConfigured: number;
	    temporaryConfigured: number;
	    personaConfigured: number;
	    effectiveCount: number;
	
	    static createFrom(source: any = {}) {
	        return new BrowserLaunchArgumentSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileConfigured = source["profileConfigured"];
	        this.temporaryConfigured = source["temporaryConfigured"];
	        this.personaConfigured = source["personaConfigured"];
	        this.effectiveCount = source["effectiveCount"];
	    }
	}
	export class BrowserLaunchNetworkSummary {
	    configuredMode: string;
	    effectiveMode: string;
	    proxyId?: string;
	    temporary: boolean;
	    connectorType: string;
	    failClosed: boolean;
	    dnsMode: string;
	    webRtcMode: string;
	    localeLinkMode: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserLaunchNetworkSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configuredMode = source["configuredMode"];
	        this.effectiveMode = source["effectiveMode"];
	        this.proxyId = source["proxyId"];
	        this.temporary = source["temporary"];
	        this.connectorType = source["connectorType"];
	        this.failClosed = source["failClosed"];
	        this.dnsMode = source["dnsMode"];
	        this.webRtcMode = source["webRtcMode"];
	        this.localeLinkMode = source["localeLinkMode"];
	    }
	}
	export class BrowserLaunchTargetSummary {
	    immediateCount: number;
	    deferredCount: number;
	    restoreLastSession: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BrowserLaunchTargetSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.immediateCount = source["immediateCount"];
	        this.deferredCount = source["deferredCount"];
	        this.restoreLastSession = source["restoreLastSession"];
	    }
	}
	export class BrowserLaunchPlanV2 {
	    schemaVersion: number;
	    profileId: string;
	    environmentId: string;
	    environmentRevision: string;
	    createdAt: string;
	    state: string;
	    kernel: BrowserKernelCapabilityManifest;
	    network: BrowserLaunchNetworkSummary;
	    arguments: BrowserLaunchArgumentSummary;
	    start: BrowserLaunchTargetSummary;
	    warnings?: string[];
	    digest: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserLaunchPlanV2(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.profileId = source["profileId"];
	        this.environmentId = source["environmentId"];
	        this.environmentRevision = source["environmentRevision"];
	        this.createdAt = source["createdAt"];
	        this.state = source["state"];
	        this.kernel = this.convertValues(source["kernel"], BrowserKernelCapabilityManifest);
	        this.network = this.convertValues(source["network"], BrowserLaunchNetworkSummary);
	        this.arguments = this.convertValues(source["arguments"], BrowserLaunchArgumentSummary);
	        this.start = this.convertValues(source["start"], BrowserLaunchTargetSummary);
	        this.warnings = source["warnings"];
	        this.digest = source["digest"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserLaunchReceiptV2 {
	    plan: BrowserLaunchPlanV2;
	    state: string;
	    startedAt?: string;
	    processStartedAt?: string;
	    debugReadyAt?: string;
	    finishedAt?: string;
	    pid?: number;
	    debugPort?: number;
	    debugReady: boolean;
	    attempt?: number;
	    maxAttempts?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserLaunchReceiptV2(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.plan = this.convertValues(source["plan"], BrowserLaunchPlanV2);
	        this.state = source["state"];
	        this.startedAt = source["startedAt"];
	        this.processStartedAt = source["processStartedAt"];
	        this.debugReadyAt = source["debugReadyAt"];
	        this.finishedAt = source["finishedAt"];
	        this.pid = source["pid"];
	        this.debugPort = source["debugPort"];
	        this.debugReady = source["debugReady"];
	        this.attempt = source["attempt"];
	        this.maxAttempts = source["maxAttempts"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class BrowserLifecycleCloneRequest {
	    profileId: string;
	    newName?: string;
	    environmentMode: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserLifecycleCloneRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.newName = source["newName"];
	        this.environmentMode = source["environmentMode"];
	    }
	}
	export class BrowserLifecycleCloneResult {
	    profile?: browser.Profile;
	    environmentId: string;
	    sourceEnvironmentId: string;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserLifecycleCloneResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], browser.Profile);
	        this.environmentId = source["environmentId"];
	        this.sourceEnvironmentId = source["sourceEnvironmentId"];
	        this.mode = source["mode"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserNetworkConnectionObservation {
	    observed: boolean;
	    network?: string;
	    remoteAddr?: string;
	    reused: boolean;
	    durationMs?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserNetworkConnectionObservation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.observed = source["observed"];
	        this.network = source["network"];
	        this.remoteAddr = source["remoteAddr"];
	        this.reused = source["reused"];
	        this.durationMs = source["durationMs"];
	        this.error = source["error"];
	    }
	}
	export class BrowserNetworkDNSObservation {
	    observed: boolean;
	    host?: string;
	    addressCount: number;
	    addressHashes?: string[];
	    families?: string[];
	    durationMs?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserNetworkDNSObservation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.observed = source["observed"];
	        this.host = source["host"];
	        this.addressCount = source["addressCount"];
	        this.addressHashes = source["addressHashes"];
	        this.families = source["families"];
	        this.durationMs = source["durationMs"];
	        this.error = source["error"];
	    }
	}
	export class BrowserNetworkEvidenceStatus {
	    status: string;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserNetworkEvidenceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.reason = source["reason"];
	    }
	}
	export class BrowserNetworkTLSObservation {
	    observed: boolean;
	    version?: string;
	    cipherSuite?: string;
	    negotiatedProtocol?: string;
	    serverName?: string;
	    durationMs?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserNetworkTLSObservation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.observed = source["observed"];
	        this.version = source["version"];
	        this.cipherSuite = source["cipherSuite"];
	        this.negotiatedProtocol = source["negotiatedProtocol"];
	        this.serverName = source["serverName"];
	        this.durationMs = source["durationMs"];
	        this.error = source["error"];
	    }
	}
	export class BrowserNetworkObservation {
	    schemaVersion: number;
	    id: string;
	    profileId?: string;
	    source: string;
	    url: string;
	    startedAt: string;
	    finishedAt: string;
	    totalMs: number;
	    ok: boolean;
	    statusCode?: number;
	    httpProtocol?: string;
	    dns: BrowserNetworkDNSObservation;
	    connection: BrowserNetworkConnectionObservation;
	    tls: BrowserNetworkTLSObservation;
	    http3: BrowserNetworkEvidenceStatus;
	    quic: BrowserNetworkEvidenceStatus;
	    ja3: BrowserNetworkEvidenceStatus;
	    ja4: BrowserNetworkEvidenceStatus;
	    browserRoute: BrowserNetworkEvidenceStatus;
	    serverSeen: BrowserNetworkEvidenceStatus;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserNetworkObservation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.profileId = source["profileId"];
	        this.source = source["source"];
	        this.url = source["url"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.totalMs = source["totalMs"];
	        this.ok = source["ok"];
	        this.statusCode = source["statusCode"];
	        this.httpProtocol = source["httpProtocol"];
	        this.dns = this.convertValues(source["dns"], BrowserNetworkDNSObservation);
	        this.connection = this.convertValues(source["connection"], BrowserNetworkConnectionObservation);
	        this.tls = this.convertValues(source["tls"], BrowserNetworkTLSObservation);
	        this.http3 = this.convertValues(source["http3"], BrowserNetworkEvidenceStatus);
	        this.quic = this.convertValues(source["quic"], BrowserNetworkEvidenceStatus);
	        this.ja3 = this.convertValues(source["ja3"], BrowserNetworkEvidenceStatus);
	        this.ja4 = this.convertValues(source["ja4"], BrowserNetworkEvidenceStatus);
	        this.browserRoute = this.convertValues(source["browserRoute"], BrowserNetworkEvidenceStatus);
	        this.serverSeen = this.convertValues(source["serverSeen"], BrowserNetworkEvidenceStatus);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserNetworkObservationRequest {
	    profileId?: string;
	    url: string;
	    timeoutMs?: number;
	
	    static createFrom(source: any = {}) {
	        return new BrowserNetworkObservationRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.url = source["url"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}
	
	export class BrowserRuntimeContextEvidence {
	    context: string;
	    applicable: boolean;
	    executed: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserRuntimeContextEvidence(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.applicable = source["applicable"];
	        this.executed = source["executed"];
	        this.error = source["error"];
	    }
	}
	export class BrowserRuntimeEvidenceField {
	    key: string;
	    capability: string;
	    contexts?: string[];
	    observed: boolean;
	    assertion: string;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserRuntimeEvidenceField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.capability = source["capability"];
	        this.contexts = source["contexts"];
	        this.observed = source["observed"];
	        this.assertion = source["assertion"];
	        this.note = source["note"];
	    }
	}
	export class BrowserRuntimeEvidenceV2 {
	    schemaVersion: number;
	    profileId: string;
	    environmentId?: string;
	    environmentRevision?: string;
	    planDigest?: string;
	    observedAt: string;
	    executionStatus: string;
	    observationStatus: string;
	    assertionStatus: string;
	    fields: BrowserRuntimeEvidenceField[];
	    contexts: BrowserRuntimeContextEvidence[];
	    issues?: string[];
	    limitations?: string[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserRuntimeEvidenceV2(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.profileId = source["profileId"];
	        this.environmentId = source["environmentId"];
	        this.environmentRevision = source["environmentRevision"];
	        this.planDigest = source["planDigest"];
	        this.observedAt = source["observedAt"];
	        this.executionStatus = source["executionStatus"];
	        this.observationStatus = source["observationStatus"];
	        this.assertionStatus = source["assertionStatus"];
	        this.fields = this.convertValues(source["fields"], BrowserRuntimeEvidenceField);
	        this.contexts = this.convertValues(source["contexts"], BrowserRuntimeContextEvidence);
	        this.issues = source["issues"];
	        this.limitations = source["limitations"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserSyncStatusV1 {
	    schemaVersion: number;
	    mode: string;
	    continuous: boolean;
	    bidirectional: boolean;
	    conflictPolicy: string;
	    localDirectory?: string;
	    schedule: Record<string, any>;
	    limitations: string[];
	
	    static createFrom(source: any = {}) {
	        return new BrowserSyncStatusV1(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.mode = source["mode"];
	        this.continuous = source["continuous"];
	        this.bidirectional = source["bidirectional"];
	        this.conflictPolicy = source["conflictPolicy"];
	        this.localDirectory = source["localDirectory"];
	        this.schedule = source["schedule"];
	        this.limitations = source["limitations"];
	    }
	}
	
	
	export class CookieInfo {
	    name: string;
	    value: string;
	    domain: string;
	    path: string;
	    expires: number;
	    httpOnly: boolean;
	    secure: boolean;
	    sameSite: string;
	
	    static createFrom(source: any = {}) {
	        return new CookieInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	        this.domain = source["domain"];
	        this.path = source["path"];
	        this.expires = source["expires"];
	        this.httpOnly = source["httpOnly"];
	        this.secure = source["secure"];
	        this.sameSite = source["sameSite"];
	    }
	}
	export class ProfilePackageExportOptions {
	    portableLogin: boolean;
	    migrationPassword: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePackageExportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.portableLogin = source["portableLogin"];
	        this.migrationPassword = source["migrationPassword"];
	    }
	}
	export class ProfilePackageExportResult {
	    cancelled: boolean;
	    zipPath: string;
	    profileCount: number;
	    fileCount: number;
	    portableLoginCount: number;
	    warnings?: string[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePackageExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cancelled = source["cancelled"];
	        this.zipPath = source["zipPath"];
	        this.profileCount = source["profileCount"];
	        this.fileCount = source["fileCount"];
	        this.portableLoginCount = source["portableLoginCount"];
	        this.warnings = source["warnings"];
	        this.message = source["message"];
	    }
	}
	export class ProfilePackageImportAction {
	    sourceProfileId: string;
	    sourceIndex: number;
	    mode: string;
	    profileName: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePackageImportAction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceProfileId = source["sourceProfileId"];
	        this.sourceIndex = source["sourceIndex"];
	        this.mode = source["mode"];
	        this.profileName = source["profileName"];
	    }
	}
	export class ProfilePackageImportConflict {
	    sourceProfileId: string;
	    sourceProfileName: string;
	    targetProfileId: string;
	    targetProfileName: string;
	    matchType: string;
	    targetRunning: boolean;
	    targetDeleted: boolean;
	    ambiguous: boolean;
	    targetMatches: number;
	    sourceTargetCollision: boolean;
	    sourceNameCollision: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePackageImportConflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceProfileId = source["sourceProfileId"];
	        this.sourceProfileName = source["sourceProfileName"];
	        this.targetProfileId = source["targetProfileId"];
	        this.targetProfileName = source["targetProfileName"];
	        this.matchType = source["matchType"];
	        this.targetRunning = source["targetRunning"];
	        this.targetDeleted = source["targetDeleted"];
	        this.ambiguous = source["ambiguous"];
	        this.targetMatches = source["targetMatches"];
	        this.sourceTargetCollision = source["sourceTargetCollision"];
	        this.sourceNameCollision = source["sourceNameCollision"];
	    }
	}
	export class ProfilePackageImportOptions {
	    actions?: ProfilePackageImportAction[];
	    conflictMode: string;
	    confirmConflict: boolean;
	    migrationPassword?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePackageImportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.actions = this.convertValues(source["actions"], ProfilePackageImportAction);
	        this.conflictMode = source["conflictMode"];
	        this.confirmConflict = source["confirmConflict"];
	        this.migrationPassword = source["migrationPassword"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProfilePackageImportPreviewProfile {
	    sourceIndex: number;
	    sourceProfileId: string;
	    sourceProfileName: string;
	    targetProfileId: string;
	    targetProfileName: string;
	    matchType: string;
	    targetRunning: boolean;
	    targetDeleted: boolean;
	    ambiguous: boolean;
	    targetMatches: number;
	    sourceTargetCollision: boolean;
	    sourceNameCollision: boolean;
	    suggestedAction: string;
	    suggestedProfileName: string;
	    canOverwrite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePackageImportPreviewProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourceIndex = source["sourceIndex"];
	        this.sourceProfileId = source["sourceProfileId"];
	        this.sourceProfileName = source["sourceProfileName"];
	        this.targetProfileId = source["targetProfileId"];
	        this.targetProfileName = source["targetProfileName"];
	        this.matchType = source["matchType"];
	        this.targetRunning = source["targetRunning"];
	        this.targetDeleted = source["targetDeleted"];
	        this.ambiguous = source["ambiguous"];
	        this.targetMatches = source["targetMatches"];
	        this.sourceTargetCollision = source["sourceTargetCollision"];
	        this.sourceNameCollision = source["sourceNameCollision"];
	        this.suggestedAction = source["suggestedAction"];
	        this.suggestedProfileName = source["suggestedProfileName"];
	        this.canOverwrite = source["canOverwrite"];
	    }
	}
	export class ProfilePackageImportPreview {
	    profiles: ProfilePackageImportPreviewProfile[];
	    cancelled: boolean;
	    zipPath: string;
	    profileCount: number;
	    conflictCount: number;
	    canOverwrite: boolean;
	    conflicts: ProfilePackageImportConflict[];
	    portableLogin: boolean;
	    portableLoginCount: number;
	    requiresMigrationPassword: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePackageImportPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profiles = this.convertValues(source["profiles"], ProfilePackageImportPreviewProfile);
	        this.cancelled = source["cancelled"];
	        this.zipPath = source["zipPath"];
	        this.profileCount = source["profileCount"];
	        this.conflictCount = source["conflictCount"];
	        this.canOverwrite = source["canOverwrite"];
	        this.conflicts = this.convertValues(source["conflicts"], ProfilePackageImportConflict);
	        this.portableLogin = source["portableLogin"];
	        this.portableLoginCount = source["portableLoginCount"];
	        this.requiresMigrationPassword = source["requiresMigrationPassword"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ProfilePackageImportResult {
	    renamedCount: number;
	    cancelled: boolean;
	    importedCount: number;
	    createdCount: number;
	    overwrittenCount: number;
	    profileMappings: Record<string, string>;
	    warnings: string[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePackageImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.renamedCount = source["renamedCount"];
	        this.cancelled = source["cancelled"];
	        this.importedCount = source["importedCount"];
	        this.createdCount = source["createdCount"];
	        this.overwrittenCount = source["overwrittenCount"];
	        this.profileMappings = source["profileMappings"];
	        this.warnings = source["warnings"];
	        this.message = source["message"];
	    }
	}
	export class ProxyBridgeWarmupResult {
	    proxyId: string;
	    ok: boolean;
	    engine: string;
	    socksUrl: string;
	    latencyMs: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyBridgeWarmupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.engine = source["engine"];
	        this.socksUrl = source["socksUrl"];
	        this.latencyMs = source["latencyMs"];
	        this.error = source["error"];
	    }
	}
	export class ProxyBrowserProbeRequest {
	    proxyId: string;
	    urls: string[];
	    concurrency: number;
	    timeoutMs: number;
	
	    static createFrom(source: any = {}) {
	        return new ProxyBrowserProbeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.urls = source["urls"];
	        this.concurrency = source["concurrency"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}
	export class ProxyBrowserProbeResult {
	    proxyId: string;
	    ok: boolean;
	    totalMs: number;
	    averageMs: number;
	    p95Ms: number;
	    bytes: number;
	    completed: number;
	    failed: number;
	    concurrency: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyBrowserProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.totalMs = source["totalMs"];
	        this.averageMs = source["averageMs"];
	        this.p95Ms = source["p95Ms"];
	        this.bytes = source["bytes"];
	        this.completed = source["completed"];
	        this.failed = source["failed"];
	        this.concurrency = source["concurrency"];
	        this.error = source["error"];
	    }
	}
	export class ProxyCoreDownloadInfoResult {
	    core: string;
	    goos: string;
	    goarch: string;
	    version: string;
	    repo: string;
	    releaseUrl: string;
	    downloadUrl: string;
	    assetName: string;
	    installDir: string;
	    binaryName: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyCoreDownloadInfoResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.core = source["core"];
	        this.goos = source["goos"];
	        this.goarch = source["goarch"];
	        this.version = source["version"];
	        this.repo = source["repo"];
	        this.releaseUrl = source["releaseUrl"];
	        this.downloadUrl = source["downloadUrl"];
	        this.assetName = source["assetName"];
	        this.installDir = source["installDir"];
	        this.binaryName = source["binaryName"];
	        this.message = source["message"];
	    }
	}
	export class ProxyCoreDownloadRequest {
	    core: string;
	    goos: string;
	    goarch: string;
	    proxyConfig: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyCoreDownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.core = source["core"];
	        this.goos = source["goos"];
	        this.goarch = source["goarch"];
	        this.proxyConfig = source["proxyConfig"];
	        this.version = source["version"];
	    }
	}
	export class ProxyCoreStatusResult {
	    core: string;
	    goos: string;
	    goarch: string;
	    installed: boolean;
	    configured: boolean;
	    active: boolean;
	    binaryPath: string;
	    source: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyCoreStatusResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.core = source["core"];
	        this.goos = source["goos"];
	        this.goarch = source["goarch"];
	        this.installed = source["installed"];
	        this.configured = source["configured"];
	        this.active = source["active"];
	        this.binaryPath = source["binaryPath"];
	        this.source = source["source"];
	        this.message = source["message"];
	    }
	}
	export class ProxyIPHealthResult {
	    proxyId: string;
	    ok: boolean;
	    source: string;
	    error: string;
	    ip: string;
	    fraudScore: number;
	    isResidential: boolean;
	    isBroadcast: boolean;
	    country: string;
	    region: string;
	    city: string;
	    asOrganization: string;
	    rawData: Record<string, any>;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyIPHealthResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.source = source["source"];
	        this.error = source["error"];
	        this.ip = source["ip"];
	        this.fraudScore = source["fraudScore"];
	        this.isResidential = source["isResidential"];
	        this.isBroadcast = source["isBroadcast"];
	        this.country = source["country"];
	        this.region = source["region"];
	        this.city = source["city"];
	        this.asOrganization = source["asOrganization"];
	        this.rawData = source["rawData"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ProxyLocationOption {
	    label: string;
	    timezone: string;
	    lang: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyLocationOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.timezone = source["timezone"];
	        this.lang = source["lang"];
	    }
	}
	export class ProxyLocationResolveResult {
	    proxyId: string;
	    ok: boolean;
	    auto: boolean;
	    source: string;
	    error: string;
	    ip: string;
	    country: string;
	    region: string;
	    city: string;
	    timezone: string;
	    lang: string;
	    health?: ProxyIPHealthResult;
	    alternates?: ProxyLocationOption[];
	    resolvedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyLocationResolveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.auto = source["auto"];
	        this.source = source["source"];
	        this.error = source["error"];
	        this.ip = source["ip"];
	        this.country = source["country"];
	        this.region = source["region"];
	        this.city = source["city"];
	        this.timezone = source["timezone"];
	        this.lang = source["lang"];
	        this.health = this.convertValues(source["health"], ProxyIPHealthResult);
	        this.alternates = this.convertValues(source["alternates"], ProxyLocationOption);
	        this.resolvedAt = source["resolvedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProxyTestResult {
	    proxyId: string;
	    ok: boolean;
	    latencyMs: number;
	    engine: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.ok = source["ok"];
	        this.latencyMs = source["latencyMs"];
	        this.engine = source["engine"];
	        this.error = source["error"];
	    }
	}
	export class ProxyValidationResult {
	    supported: boolean;
	    errorMsg: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.errorMsg = source["errorMsg"];
	    }
	}
	export class SnapshotInfo {
	    snapshotId: string;
	    profileId: string;
	    name: string;
	    sizeMB: number;
	    createdAt: string;
	    filePath?: string;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snapshotId = source["snapshotId"];
	        this.profileId = source["profileId"];
	        this.name = source["name"];
	        this.sizeMB = source["sizeMB"];
	        this.createdAt = source["createdAt"];
	        this.filePath = source["filePath"];
	    }
	}

}

export namespace backup {
	
	export class ManifestEntry {
	    id: string;
	    category: string;
	    entryType: string;
	    required: boolean;
	    coreId?: string;
	    archivePath: string;
	    fileCount?: number;
	    byteSize?: number;
	    sha256?: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new ManifestEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.entryType = source["entryType"];
	        this.required = source["required"];
	        this.coreId = source["coreId"];
	        this.archivePath = source["archivePath"];
	        this.fileCount = source["fileCount"];
	        this.byteSize = source["byteSize"];
	        this.sha256 = source["sha256"];
	        this.description = source["description"];
	    }
	}
	export class ManifestAppInfo {
	    name: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new ManifestAppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	    }
	}
	export class Manifest {
	    format: string;
	    manifestVersion: number;
	    createdAt: string;
	    app: ManifestAppInfo;
	    entries: ManifestEntry[];
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.manifestVersion = source["manifestVersion"];
	        this.createdAt = source["createdAt"];
	        this.app = this.convertValues(source["app"], ManifestAppInfo);
	        this.entries = this.convertValues(source["entries"], ManifestEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class ScopeEntry {
	    id: string;
	    category: string;
	    entryType: string;
	    required: boolean;
	    coreId?: string;
	    sourcePath: string;
	    archivePath: string;
	    exists: boolean;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScopeEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.entryType = source["entryType"];
	        this.required = source["required"];
	        this.coreId = source["coreId"];
	        this.sourcePath = source["sourcePath"];
	        this.archivePath = source["archivePath"];
	        this.exists = source["exists"];
	        this.description = source["description"];
	    }
	}
	export class Scope {
	    format: string;
	    manifestVersion: number;
	    appRoot: string;
	    entries: ScopeEntry[];
	
	    static createFrom(source: any = {}) {
	        return new Scope(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.manifestVersion = source["manifestVersion"];
	        this.appRoot = source["appRoot"];
	        this.entries = this.convertValues(source["entries"], ScopeEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace browser {
	
	export class CoreExtendedInfo {
	    coreId: string;
	    chromeVersion: string;
	    instanceCount: number;
	
	    static createFrom(source: any = {}) {
	        return new CoreExtendedInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreId = source["coreId"];
	        this.chromeVersion = source["chromeVersion"];
	        this.instanceCount = source["instanceCount"];
	    }
	}
	export class CoreInput {
	    coreId: string;
	    coreName: string;
	    corePath: string;
	    isDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CoreInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreId = source["coreId"];
	        this.coreName = source["coreName"];
	        this.corePath = source["corePath"];
	        this.isDefault = source["isDefault"];
	    }
	}
	export class CoreValidateResult {
	    valid: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CoreValidateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.message = source["message"];
	    }
	}
	export class EnvironmentContractIdentity {
	    environmentId: string;
	    profileId: string;
	    revision: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentContractIdentity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.environmentId = source["environmentId"];
	        this.profileId = source["profileId"];
	        this.revision = source["revision"];
	        this.source = source["source"];
	    }
	}
	export class EnvironmentLegacyExtras {
	    hasFingerprintArgs: boolean;
	    fingerprintArgCount: number;
	    hasLaunchArgs: boolean;
	    launchArgCount: number;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentLegacyExtras(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasFingerprintArgs = source["hasFingerprintArgs"];
	        this.fingerprintArgCount = source["fingerprintArgCount"];
	        this.hasLaunchArgs = source["hasLaunchArgs"];
	        this.launchArgCount = source["launchArgCount"];
	    }
	}
	export class EnvironmentRuntimeState {
	    state: string;
	    running: boolean;
	    debugReady: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentRuntimeState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.running = source["running"];
	        this.debugReady = source["debugReady"];
	    }
	}
	export class EnvironmentStorageBinding {
	    managedUserDataDir: boolean;
	    restoreLastSession: string;
	    memoryLimitMb: number;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentStorageBinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.managedUserDataDir = source["managedUserDataDir"];
	        this.restoreLastSession = source["restoreLastSession"];
	        this.memoryLimitMb = source["memoryLimitMb"];
	    }
	}
	export class EnvironmentTemplateBinding {
	    profileId: string;
	    templateId: string;
	    templateRevision: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentTemplateBinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.templateId = source["templateId"];
	        this.templateRevision = source["templateRevision"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class NetworkPolicy {
	    profileId: string;
	    failClosed: boolean;
	    dnsMode: string;
	    webRtcMode: string;
	    localeLinkMode: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkPolicy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.failClosed = source["failClosed"];
	        this.dnsMode = source["dnsMode"];
	        this.webRtcMode = source["webRtcMode"];
	        this.localeLinkMode = source["localeLinkMode"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class EnvironmentNetworkBinding {
	    mode: string;
	    proxyId?: string;
	    proxyBindName?: string;
	    connectorType: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentNetworkBinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.proxyId = source["proxyId"];
	        this.proxyBindName = source["proxyBindName"];
	        this.connectorType = source["connectorType"];
	    }
	}
	export class EnvironmentKernelBinding {
	    configuredCoreId?: string;
	    effectiveCoreId?: string;
	    effectiveCoreName?: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentKernelBinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configuredCoreId = source["configuredCoreId"];
	        this.effectiveCoreId = source["effectiveCoreId"];
	        this.effectiveCoreName = source["effectiveCoreName"];
	        this.status = source["status"];
	    }
	}
	export class EnvironmentPersona {
	    name: string;
	    tags: string[];
	    keywords: string[];
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentPersona(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.tags = source["tags"];
	        this.keywords = source["keywords"];
	    }
	}
	export class EnvironmentContractV2 {
	    schemaVersion: number;
	    environment: EnvironmentContractIdentity;
	    persona: EnvironmentPersona;
	    kernelBinding: EnvironmentKernelBinding;
	    networkBinding: EnvironmentNetworkBinding;
	    networkPolicy: NetworkPolicy;
	    template?: EnvironmentTemplateBinding;
	    storage: EnvironmentStorageBinding;
	    runtime: EnvironmentRuntimeState;
	    legacyExtras: EnvironmentLegacyExtras;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentContractV2(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.environment = this.convertValues(source["environment"], EnvironmentContractIdentity);
	        this.persona = this.convertValues(source["persona"], EnvironmentPersona);
	        this.kernelBinding = this.convertValues(source["kernelBinding"], EnvironmentKernelBinding);
	        this.networkBinding = this.convertValues(source["networkBinding"], EnvironmentNetworkBinding);
	        this.networkPolicy = this.convertValues(source["networkPolicy"], NetworkPolicy);
	        this.template = this.convertValues(source["template"], EnvironmentTemplateBinding);
	        this.storage = this.convertValues(source["storage"], EnvironmentStorageBinding);
	        this.runtime = this.convertValues(source["runtime"], EnvironmentRuntimeState);
	        this.legacyExtras = this.convertValues(source["legacyExtras"], EnvironmentLegacyExtras);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EnvironmentIdentity {
	    profileId: string;
	    environmentId: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentIdentity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.environmentId = source["environmentId"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	
	
	
	
	
	
	export class EnvironmentTemplateRule {
	    key: string;
	    enforcement: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentTemplateRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.enforcement = source["enforcement"];
	        this.description = source["description"];
	    }
	}
	export class EnvironmentTemplate {
	    schemaVersion: number;
	    id: string;
	    revision: string;
	    name: string;
	    description: string;
	    targetOS?: string;
	    deviceClass?: string;
	    kernelFamily?: string;
	    supportLevel?: string;
	    requiredCapabilities?: string[];
	    restoreLastSession: string;
	    memoryLimitMb: number;
	    rules: EnvironmentTemplateRule[];
	
	    static createFrom(source: any = {}) {
	        return new EnvironmentTemplate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.revision = source["revision"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.targetOS = source["targetOS"];
	        this.deviceClass = source["deviceClass"];
	        this.kernelFamily = source["kernelFamily"];
	        this.supportLevel = source["supportLevel"];
	        this.requiredCapabilities = source["requiredCapabilities"];
	        this.restoreLastSession = source["restoreLastSession"];
	        this.memoryLimitMb = source["memoryLimitMb"];
	        this.rules = this.convertValues(source["rules"], EnvironmentTemplateRule);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class Extension {
	    extensionId: string;
	    name: string;
	    version: string;
	    description: string;
	    iconDataUrl: string;
	    manifestJson: string;
	    sourceUrl: string;
	    installDir: string;
	    installMode: string;
	    packagePath: string;
	    packageHash: string;
	    enabled: boolean;
	    defaultInstall: boolean;
	    installedAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Extension(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extensionId = source["extensionId"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.iconDataUrl = source["iconDataUrl"];
	        this.manifestJson = source["manifestJson"];
	        this.sourceUrl = source["sourceUrl"];
	        this.installDir = source["installDir"];
	        this.installMode = source["installMode"];
	        this.packagePath = source["packagePath"];
	        this.packageHash = source["packageHash"];
	        this.enabled = source["enabled"];
	        this.defaultInstall = source["defaultInstall"];
	        this.installedAt = source["installedAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ExtensionLookupResult {
	    extensionId: string;
	    name: string;
	    version: string;
	    description: string;
	    storeUrl: string;
	    installable: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionLookupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extensionId = source["extensionId"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.storeUrl = source["storeUrl"];
	        this.installable = source["installable"];
	        this.message = source["message"];
	    }
	}
	export class ExtensionSearchResult {
	    extensionId: string;
	    name: string;
	    storeUrl: string;
	    iconUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extensionId = source["extensionId"];
	        this.name = source["name"];
	        this.storeUrl = source["storeUrl"];
	        this.iconUrl = source["iconUrl"];
	    }
	}
	export class Group {
	    groupId: string;
	    groupName: string;
	    parentId: string;
	    sortOrder: number;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupId = source["groupId"];
	        this.groupName = source["groupName"];
	        this.parentId = source["parentId"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class GroupInput {
	    groupName: string;
	    parentId: string;
	    sortOrder: number;
	
	    static createFrom(source: any = {}) {
	        return new GroupInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupName = source["groupName"];
	        this.parentId = source["parentId"];
	        this.sortOrder = source["sortOrder"];
	    }
	}
	export class GroupWithCount {
	    groupId: string;
	    groupName: string;
	    parentId: string;
	    sortOrder: number;
	    createdAt: string;
	    updatedAt: string;
	    instanceCount: number;
	
	    static createFrom(source: any = {}) {
	        return new GroupWithCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupId = source["groupId"];
	        this.groupName = source["groupName"];
	        this.parentId = source["parentId"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.instanceCount = source["instanceCount"];
	    }
	}
	
	export class Profile {
	    profileId: string;
	    profileName: string;
	    userDataDir: string;
	    coreId: string;
	    restoreLastSession: string;
	    fingerprintArgs: string[];
	    proxyId: string;
	    proxyConfig: string;
	    proxyBindSourceId: string;
	    proxyBindSourceUrl: string;
	    proxyBindName: string;
	    proxyBindUpdatedAt: string;
	    memoryLimitMb: number;
	    launchArgs: string[];
	    lastLaunchArgs: string[];
	    tags: string[];
	    keywords: string[];
	    groupId: string;
	    launchCode: string;
	    windowMarkerCode?: string;
	    runtimeState: string;
	    running: boolean;
	    debugPort: number;
	    debugReady: boolean;
	    pid: number;
	    runtimeWarning: string;
	    lastError: string;
	    createdAt: string;
	    updatedAt: string;
	    deletedAt: string;
	    lastStartAt: string;
	    lastStopAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.userDataDir = source["userDataDir"];
	        this.coreId = source["coreId"];
	        this.restoreLastSession = source["restoreLastSession"];
	        this.fingerprintArgs = source["fingerprintArgs"];
	        this.proxyId = source["proxyId"];
	        this.proxyConfig = source["proxyConfig"];
	        this.proxyBindSourceId = source["proxyBindSourceId"];
	        this.proxyBindSourceUrl = source["proxyBindSourceUrl"];
	        this.proxyBindName = source["proxyBindName"];
	        this.proxyBindUpdatedAt = source["proxyBindUpdatedAt"];
	        this.memoryLimitMb = source["memoryLimitMb"];
	        this.launchArgs = source["launchArgs"];
	        this.lastLaunchArgs = source["lastLaunchArgs"];
	        this.tags = source["tags"];
	        this.keywords = source["keywords"];
	        this.groupId = source["groupId"];
	        this.launchCode = source["launchCode"];
	        this.windowMarkerCode = source["windowMarkerCode"];
	        this.runtimeState = source["runtimeState"];
	        this.running = source["running"];
	        this.debugPort = source["debugPort"];
	        this.debugReady = source["debugReady"];
	        this.pid = source["pid"];
	        this.runtimeWarning = source["runtimeWarning"];
	        this.lastError = source["lastError"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.deletedAt = source["deletedAt"];
	        this.lastStartAt = source["lastStartAt"];
	        this.lastStopAt = source["lastStopAt"];
	    }
	}
	export class ProfileCopyOptions {
	    mode: string;
	    automationTargets: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProfileCopyOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.automationTargets = source["automationTargets"];
	    }
	}
	export class ProfileExtensionSettings {
	    profileId: string;
	    configured: boolean;
	    extensionIds: string[];
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfileExtensionSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.configured = source["configured"];
	        this.extensionIds = source["extensionIds"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ProfileInput {
	    profileName: string;
	    userDataDir: string;
	    coreId: string;
	    restoreLastSession: string;
	    fingerprintArgs: string[];
	    proxyId: string;
	    proxyConfig: string;
	    memoryLimitMb: number;
	    launchArgs: string[];
	    tags: string[];
	    keywords: string[];
	    groupId: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileName = source["profileName"];
	        this.userDataDir = source["userDataDir"];
	        this.coreId = source["coreId"];
	        this.restoreLastSession = source["restoreLastSession"];
	        this.fingerprintArgs = source["fingerprintArgs"];
	        this.proxyId = source["proxyId"];
	        this.proxyConfig = source["proxyConfig"];
	        this.memoryLimitMb = source["memoryLimitMb"];
	        this.launchArgs = source["launchArgs"];
	        this.tags = source["tags"];
	        this.keywords = source["keywords"];
	        this.groupId = source["groupId"];
	    }
	}
	export class ProfilePatchInput {
	    profileName?: string;
	    userDataDir?: string;
	    coreId?: string;
	    restoreLastSession?: string;
	    proxyId?: string;
	    groupId?: string;
	    tags?: string[];
	    keywords?: string[];
	    memoryLimitMb?: number;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePatchInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileName = source["profileName"];
	        this.userDataDir = source["userDataDir"];
	        this.coreId = source["coreId"];
	        this.restoreLastSession = source["restoreLastSession"];
	        this.proxyId = source["proxyId"];
	        this.groupId = source["groupId"];
	        this.tags = source["tags"];
	        this.keywords = source["keywords"];
	        this.memoryLimitMb = source["memoryLimitMb"];
	    }
	}
	export class Settings {
	    userDataRoot: string;
	    defaultFingerprintArgs: string[];
	    defaultLaunchArgs: string[];
	    defaultStartUrls: string[];
	    lightStartEnabled: boolean;
	    restoreLastSession: boolean;
	    startReadyTimeoutMs: number;
	    startStableWindowMs: number;
	    defaultConnectorType: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userDataRoot = source["userDataRoot"];
	        this.defaultFingerprintArgs = source["defaultFingerprintArgs"];
	        this.defaultLaunchArgs = source["defaultLaunchArgs"];
	        this.defaultStartUrls = source["defaultStartUrls"];
	        this.lightStartEnabled = source["lightStartEnabled"];
	        this.restoreLastSession = source["restoreLastSession"];
	        this.startReadyTimeoutMs = source["startReadyTimeoutMs"];
	        this.startStableWindowMs = source["startStableWindowMs"];
	        this.defaultConnectorType = source["defaultConnectorType"];
	    }
	}
	export class Tab {
	    tabId: string;
	    title: string;
	    url: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Tab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tabId = source["tabId"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.active = source["active"];
	    }
	}

}

export namespace config {
	
	export class BrowserBookmark {
	    name: string;
	    url: string;
	    openOnStart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BrowserBookmark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.openOnStart = source["openOnStart"];
	    }
	}
	export class BrowserCore {
	    coreId: string;
	    coreName: string;
	    corePath: string;
	    isDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BrowserCore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreId = source["coreId"];
	        this.coreName = source["coreName"];
	        this.corePath = source["corePath"];
	        this.isDefault = source["isDefault"];
	    }
	}
	export class BrowserProxy {
	    proxyId: string;
	    proxyName: string;
	    proxyConfig: string;
	    preferredKernel?: string;
	    dnsServers?: string;
	    groupName?: string;
	    sortOrder?: number;
	    sourceId?: string;
	    sourceUrl?: string;
	    sourceNamePrefix?: string;
	    sourceAutoRefresh?: boolean;
	    sourceRefreshIntervalM?: number;
	    sourceLastRefreshAt?: string;
	    lastLatencyMs: number;
	    lastTestOk: boolean;
	    lastTestedAt: string;
	    lastIPHealthJson?: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserProxy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.proxyName = source["proxyName"];
	        this.proxyConfig = source["proxyConfig"];
	        this.preferredKernel = source["preferredKernel"];
	        this.dnsServers = source["dnsServers"];
	        this.groupName = source["groupName"];
	        this.sortOrder = source["sortOrder"];
	        this.sourceId = source["sourceId"];
	        this.sourceUrl = source["sourceUrl"];
	        this.sourceNamePrefix = source["sourceNamePrefix"];
	        this.sourceAutoRefresh = source["sourceAutoRefresh"];
	        this.sourceRefreshIntervalM = source["sourceRefreshIntervalM"];
	        this.sourceLastRefreshAt = source["sourceLastRefreshAt"];
	        this.lastLatencyMs = source["lastLatencyMs"];
	        this.lastTestOk = source["lastTestOk"];
	        this.lastTestedAt = source["lastTestedAt"];
	        this.lastIPHealthJson = source["lastIPHealthJson"];
	    }
	}
	export class ProxyCheckTarget {
	    id: string;
	    name: string;
	    type: string;
	    url: string;
	    parser?: string;
	    timeoutMs?: number;
	    expectedStatus?: number[];
	
	    static createFrom(source: any = {}) {
	        return new ProxyCheckTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.parser = source["parser"];
	        this.timeoutMs = source["timeoutMs"];
	        this.expectedStatus = source["expectedStatus"];
	    }
	}
	export class ProxyCheckConfig {
	    bridgeStartTimeoutMs: number;
	    speedTargetId: string;
	    ipHealthTargetId: string;
	    targets: ProxyCheckTarget[];
	
	    static createFrom(source: any = {}) {
	        return new ProxyCheckConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bridgeStartTimeoutMs = source["bridgeStartTimeoutMs"];
	        this.speedTargetId = source["speedTargetId"];
	        this.ipHealthTargetId = source["ipHealthTargetId"];
	        this.targets = this.convertValues(source["targets"], ProxyCheckTarget);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace launchcode {
	
	export class LaunchRequestParams {
	    launchArgs: string[];
	    startUrls: string[];
	    skipDefaultStartUrls: boolean;
	    proxyId: string;
	    proxyConfig: string;
	
	    static createFrom(source: any = {}) {
	        return new LaunchRequestParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.launchArgs = source["launchArgs"];
	        this.startUrls = source["startUrls"];
	        this.skipDefaultStartUrls = source["skipDefaultStartUrls"];
	        this.proxyId = source["proxyId"];
	        this.proxyConfig = source["proxyConfig"];
	    }
	}

}

export namespace logger {
	
	export class MemoryLogEntry {
	    time: string;
	    level: string;
	    component: string;
	    message: string;
	    fields?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new MemoryLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.level = source["level"];
	        this.component = source["component"];
	        this.message = source["message"];
	        this.fields = source["fields"];
	    }
	}
	export class MethodInterceptor {
	
	
	    static createFrom(source: any = {}) {
	        return new MethodInterceptor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace proxy {
	
	export class DnsDiagnosticSummary {
	    hasConfig: boolean;
	    sourceFormat: string;
	    enhancedMode: string;
	    nameserverCount: number;
	    fallbackCount: number;
	    xrayServerCount: number;
	    unsupported: string[];
	
	    static createFrom(source: any = {}) {
	        return new DnsDiagnosticSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasConfig = source["hasConfig"];
	        this.sourceFormat = source["sourceFormat"];
	        this.enhancedMode = source["enhancedMode"];
	        this.nameserverCount = source["nameserverCount"];
	        this.fallbackCount = source["fallbackCount"];
	        this.xrayServerCount = source["xrayServerCount"];
	        this.unsupported = source["unsupported"];
	    }
	}
	export class ProxyRuntimeDiagnostic {
	    workDir: string;
	    configPath: string;
	    stderrPath: string;
	    logPath: string;
	    errorPath: string;
	    recentLogs: Record<string, string>;
	    bridgeAlive: boolean;
	    bridgePort: number;
	    lastError: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyRuntimeDiagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workDir = source["workDir"];
	        this.configPath = source["configPath"];
	        this.stderrPath = source["stderrPath"];
	        this.logPath = source["logPath"];
	        this.errorPath = source["errorPath"];
	        this.recentLogs = source["recentLogs"];
	        this.bridgeAlive = source["bridgeAlive"];
	        this.bridgePort = source["bridgePort"];
	        this.lastError = source["lastError"];
	    }
	}
	export class ProxyBuildDiagnostic {
	    proxyId: string;
	    proxyName: string;
	    found: boolean;
	    ok: boolean;
	    engine: string;
	    nodeKey: string;
	    egressMode?: string;
	    egressInterface?: string;
	    rawConfigMasked: string;
	    dnsServers: string;
	    dnsSummary: DnsDiagnosticSummary;
	    standardProxy: string;
	    outbounds: any[];
	    routes: any[];
	    inbound: Record<string, any>;
	    outbound: Record<string, any>;
	    runtime: ProxyRuntimeDiagnostic;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProxyBuildDiagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.proxyName = source["proxyName"];
	        this.found = source["found"];
	        this.ok = source["ok"];
	        this.engine = source["engine"];
	        this.nodeKey = source["nodeKey"];
	        this.egressMode = source["egressMode"];
	        this.egressInterface = source["egressInterface"];
	        this.rawConfigMasked = source["rawConfigMasked"];
	        this.dnsServers = source["dnsServers"];
	        this.dnsSummary = this.convertValues(source["dnsSummary"], DnsDiagnosticSummary);
	        this.standardProxy = source["standardProxy"];
	        this.outbounds = source["outbounds"];
	        this.routes = source["routes"];
	        this.inbound = source["inbound"];
	        this.outbound = source["outbound"];
	        this.runtime = this.convertValues(source["runtime"], ProxyRuntimeDiagnostic);
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

