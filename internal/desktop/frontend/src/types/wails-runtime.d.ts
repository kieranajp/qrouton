// The workbench process serves /wails/runtime.js at runtime, so nothing on disk
// declares it. jsconfig maps that URL here; only the surface the pages actually
// use is written down.
export declare const Call: {
  ByName(name: string, ...args: unknown[]): Promise<any>;
};

export declare const Events: {
  On(event: string, handler: (event: { data: any }) => void): () => void;
};

export declare const Browser: {
  OpenURL(url: string): Promise<void>;
};
