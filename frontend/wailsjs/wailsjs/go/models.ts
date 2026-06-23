export namespace main {
	
	export class PeerInfo {
	    steamID: string;
	    ip: string;
	
	    static createFrom(source: any = {}) {
	        return new PeerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.steamID = source["steamID"];
	        this.ip = source["ip"];
	    }
	}
	export class StatusPayload {
	    running: boolean;
	    localIP: string;
	    steamID: string;
	    peerCount: number;
	
	    static createFrom(source: any = {}) {
	        return new StatusPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.localIP = source["localIP"];
	        this.steamID = source["steamID"];
	        this.peerCount = source["peerCount"];
	    }
	}

}

