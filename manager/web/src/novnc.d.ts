declare module '@novnc/novnc' {
  export default class RFB {
    constructor(target: HTMLElement, url: string);
    scaleViewport: boolean;
    resizeSession: boolean;
    addEventListener(type: 'connect' | 'disconnect', listener: (event: Event & { detail: { clean: boolean } }) => void): void;
    disconnect(): void;
  }
}
