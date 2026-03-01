// Clip Bridge 跨端兼容封装
// iOS (WKWebView) 和 Desktop (Electron) 签名不同

const isIOS = () => !!window.webkit;

/**
 * 非流式调用 Pinix RPC
 */
export async function pinixInvoke(command, stdin = '{}', args = []) {
  if (isIOS()) {
    return Bridge.invoke('invoke', { name: command, args, stdin });
  } else if (window.Bridge) {
    return Bridge.invoke(command, { args, stdin });
  }
  // 开发模式 fallback：无 Bridge 时静默返回
  console.warn('[bridge] no Bridge available, returning mock');
  return { stdout: '[]', stderr: '', exitCode: 0 };
}

/**
 * 流式调用 Pinix RPC
 */
export function pinixInvokeStream(command, stdin, onChunk, onDone) {
  if (isIOS() && window.Bridge) {
    Bridge.invokeStream(
      'invoke',
      { name: command, args: [], stdin },
      onChunk,
      onDone
    );
  } else if (window.Bridge) {
    Bridge.invokeStream(
      command,
      { stdin },
      onChunk,
      onDone
    );
  } else {
    console.warn('[bridge] no Bridge available for stream');
    onDone(1);
  }
}
