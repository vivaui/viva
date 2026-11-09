export interface Nav {
  ID: string;
  Title: string;
  Category: string;
}

export interface Demo {
  ID: string;
  Title: string;
  Description: string;
  Snippet?: string;
  Height?: number;
}

export interface Section {
  ID: string;
  Title: string;
  Text: string;
  Items: string[] | null;
}

export interface Capability {
  Platform: string;
  Status: string;
  Detail: string;
}

export interface Page extends Nav {
  Summary: string;
  Revision: string;
  /** The Go file the examples call: components/<id>.go, or ui/list.go for a page-layer control. */
  Source: string;
  All: Nav[];
  Examples: Demo[];
  InstallCommand: string;
  Sections: Section[];
  Capabilities: Capability[];
}

export interface Coverage {
  id: string;
  file: string;
  documented: boolean;
}

export interface Catalog {
  pages: Page[];
  revision: string;
  coverage: Coverage[];
}

export type Theme = 'light' | 'dark';
export type PreviewState = 'loading' | 'ready' | 'error';

export interface PreviewUpdate {
  state: PreviewState;
  message: string;
}
