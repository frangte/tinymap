/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** multiplayer socket when it is not on the page's own host, e.g. wss://server.tinymap.live/ws (see src/net/client.ts) */
  readonly VITE_WS_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
