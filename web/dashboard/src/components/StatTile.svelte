<script lang="ts">
  import type { Snippet } from 'svelte';

  // Rich value/sub content is delivered as Svelte snippets (`children`,
  // `subSnippet`) rather than raw HTML strings. The previous raw-HTML value/
  // sub props were an opt-in XSS sink on a shared component - inert while
  // only closed server-controlled values flowed through it, but a footgun the
  // moment an endpoint or model name did. Snippets compose into the DOM as
  // normal, escaped content.
  interface Props {
    label: string;
    value?: string;
    sub?: string;
    children?: Snippet<[]>;
    subSnippet?: Snippet<[]>;
  }
