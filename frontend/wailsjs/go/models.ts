export namespace model {
	
	export class Tag {
	    id: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Tag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class Photo {
	    id: number;
	    fileName: string;
	    dest: string;
	    backupPath: string;
	    caption: string;
	    analysisState: string;
	    // Go type: time
	    backedUpAt: any;
	    tags: Tag[];
	
	    static createFrom(source: any = {}) {
	        return new Photo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.fileName = source["fileName"];
	        this.dest = source["dest"];
	        this.backupPath = source["backupPath"];
	        this.caption = source["caption"];
	        this.analysisState = source["analysisState"];
	        this.backedUpAt = this.convertValues(source["backedUpAt"], null);
	        this.tags = this.convertValues(source["tags"], Tag);
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
	
	export class TagCount {
	    name: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new TagCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.count = source["count"];
	    }
	}

}

export namespace service {
	
	export class AnalysisMetrics {
	    requests: number;
	    inputTokens: number;
	    outputTokens: number;
	    tokensReported: boolean;
	    active: number;
	    concurrency: number;
	    waitSeconds: number;
	    apiSeconds: number;
	    phase: string;
	    prepared: number;
	    cached: number;
	    elapsedSeconds: number;
	    photosPerMinute: number;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requests = source["requests"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.tokensReported = source["tokensReported"];
	        this.active = source["active"];
	        this.concurrency = source["concurrency"];
	        this.waitSeconds = source["waitSeconds"];
	        this.apiSeconds = source["apiSeconds"];
	        this.phase = source["phase"];
	        this.prepared = source["prepared"];
	        this.cached = source["cached"];
	        this.elapsedSeconds = source["elapsedSeconds"];
	        this.photosPerMinute = source["photosPerMinute"];
	    }
	}
	export class AnalysisStatus {
	    batchSize: number;
	    workers: number;
	    rpm: number;
	    tpm: number;
	    configured: boolean;
	    model: string;
	    total: number;
	    analyzed: number;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.batchSize = source["batchSize"];
	        this.workers = source["workers"];
	        this.rpm = source["rpm"];
	        this.tpm = source["tpm"];
	        this.configured = source["configured"];
	        this.model = source["model"];
	        this.total = source["total"];
	        this.analyzed = source["analyzed"];
	    }
	}
	export class FileResult {
	    name: string;
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new FileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class AnalyzeResult {
	    metrics: AnalysisMetrics;
	    done: number;
	    failed: number;
	    cancelled: boolean;
	    items: FileResult[];
	
	    static createFrom(source: any = {}) {
	        return new AnalyzeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.metrics = this.convertValues(source["metrics"], AnalysisMetrics);
	        this.done = source["done"];
	        this.failed = source["failed"];
	        this.cancelled = source["cancelled"];
	        this.items = this.convertValues(source["items"], FileResult);
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
	export class CleanAllResult {
	    photos: number;
	    tags: number;
	    settings: number;
	
	    static createFrom(source: any = {}) {
	        return new CleanAllResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.photos = source["photos"];
	        this.tags = source["tags"];
	        this.settings = source["settings"];
	    }
	}
	export class DeleteResult {
	    deleted: number;
	    failed: number;
	    items: FileResult[];
	
	    static createFrom(source: any = {}) {
	        return new DeleteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deleted = source["deleted"];
	        this.failed = source["failed"];
	        this.items = this.convertValues(source["items"], FileResult);
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
	export class IntegrityItem {
	    filename: string;
	    ok: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IntegrityItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filename = source["filename"];
	        this.ok = source["ok"];
	    }
	}
	export class DestInfo {
	    path: string;
	    integrity: IntegrityItem[];
	    warning: string;
	
	    static createFrom(source: any = {}) {
	        return new DestInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.integrity = this.convertValues(source["integrity"], IntegrityItem);
	        this.warning = source["warning"];
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
	export class FileInfo {
	    name: string;
	    size: number;
	    modTime: string;
	    inDest: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	        this.inDest = source["inDest"];
	    }
	}
	
	
	export class MoveResult {
	    moved: number;
	    skipped: number;
	    failed: number;
	    seconds: number;
	    items: FileResult[];
	
	    static createFrom(source: any = {}) {
	        return new MoveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.moved = source["moved"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.seconds = source["seconds"];
	        this.items = this.convertValues(source["items"], FileResult);
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
	export class SourceInfo {
	    path: string;
	    count: number;
	    warning: string;
	
	    static createFrom(source: any = {}) {
	        return new SourceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.count = source["count"];
	        this.warning = source["warning"];
	    }
	}
	export class State {
	    source: string;
	    dest: string;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.dest = source["dest"];
	    }
	}

}

