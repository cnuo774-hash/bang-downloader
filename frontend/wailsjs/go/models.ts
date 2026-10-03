export namespace config {

	export class PublicConfig {
	    output: string;
	    maxDownload: string;
	    engineVersion: string;

	    static createFrom(source: any = {}) {
	        return new PublicConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.output = source["output"];
	        this.maxDownload = source["maxDownload"];
	        this.engineVersion = source["engineVersion"];
	    }
	}

}

export namespace engine {

	export class Task {
	    id: string;
	    name: string;
	    status: string;
	    total: number;
	    completed: number;
	    downloadBps: number;
	    output: string;
	    error?: string;
	    updatedAt: number;
	    addedAt: number;
	    followedBy?: string[];

	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.total = source["total"];
	        this.completed = source["completed"];
	        this.downloadBps = source["downloadBps"];
	        this.output = source["output"];
	        this.error = source["error"];
	        this.updatedAt = source["updatedAt"];
	        this.addedAt = source["addedAt"];
	        this.followedBy = source["followedBy"];
	    }
	}
	export class TaskStats {
	    active: number;
	    complete: number;
	    downloadBps: number;

	    static createFrom(source: any = {}) {
	        return new TaskStats(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.complete = source["complete"];
	        this.downloadBps = source["downloadBps"];
	    }
	}
	export class TaskPage {
	    items: Task[];
	    total: number;
	    revision: number;
	    stats: TaskStats;

	    static createFrom(source: any = {}) {
	        return new TaskPage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Task);
	        this.total = source["total"];
	        this.revision = source["revision"];
	        this.stats = this.convertValues(source["stats"], TaskStats);
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
