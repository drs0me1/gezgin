export {};

declare global {
  interface HTMLElement {
    // TODO: no idea what the exact type is
    __vue__: any;
  }

  interface HTMLElement {
    clickOutsideEvent?: (event: Event) => void;
  }
}
