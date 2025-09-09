<script>
  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import { StartBackend, StopBackend, ToggleFRPC, UpdateHtmlTitle } from '../wailsjs/go/main/App.js';
  import { EventsOn, WindowMinimise, WindowToggleMaximise, Quit } from '../wailsjs/runtime/runtime.js';
  import { locale, t } from './lib/i18n/store.js';

  let services = {
    backend: { name: 'Backend', status: 'Stopped' },
    frpc: { name: 'FRPC', status: 'Stopped' }
  };
  let streamKey = 'N/A';
  let logs = '';
  let logTextarea;
  let logsVisible = false;
  let htmlTitle = '';

  function updateTitle() {
    if (htmlTitle.trim() !== '') {
      UpdateHtmlTitle(htmlTitle);
      logs += `[App] ${$t('titleUpdated', { newTitle: htmlTitle })}\n`;
      scrollToBottom();
    }
  }

  function toggleLocale() {
    locale.update(l => l === 'en' ? 'zh' : 'en');
  }

  $: isAnyServiceRunning = Object.values(services).some(s => s.status === 'Running');

  function scrollToBottom() {
    if (logTextarea) {
      logTextarea.scrollTop = logTextarea.scrollHeight;
    }
  }

  onMount(() => {
    logs = $t('welcomeMessage') + '\n';
    EventsOn('service:status', (name, status) => {
      if (name === 'livego' || name === 'caddy') {
        if (status === 'Running') {
          services.backend.status = 'Running';
        } else {
          // Only set to stopped if both are stopped
          if (services.livego?.status !== 'Running' && services.caddy?.status !== 'Running') {
            services.backend.status = 'Stopped';
          }
        }
        services = services; // Trigger reactivity
      } else if (services[name]) {
        services[name].status = status;
        services = services; // Trigger reactivity
      }
      logs += $t('statusChanged', { serviceName: $t(`services.${name.toLowerCase()}`), status: status }) + '\n';
      scrollToBottom();
    });

    EventsOn('service:log', (name, message) => {
      logs += `[${name}] ${message}\n`;
      scrollToBottom();
    });

    EventsOn('streamkey:update', (key) => {
      streamKey = key;
    });

    EventsOn('title:updated', (newTitle) => {
      showToast($t('titleUpdatedToast', { newTitle: newTitle }));
    });
  });

  function toggleBackend() {
    if (services.backend.status === 'Running') {
      logs += $t('stoppingServices') + '\n';
      StopBackend();
    } else {
      logs += $t('startingServices') + '\n';
      StartBackend();
    }
    scrollToBottom();
  }

  function toggleFRPC() {
    logs += `[App] Toggling FRPC...\n`;
    ToggleFRPC();
    scrollToBottom();
  }

  let copyButtonText = {
    'server-url': 'Copy',
    'stream-key': 'Copy'
  };
  
  $: {
      copyButtonText = {
          'server-url': $t('copy'),
          'stream-key': $t('copy')
      }
  }

  function copyToClipboard(elementId) {
    const input = document.getElementById(elementId);
    if (input instanceof HTMLInputElement && navigator.clipboard) {
      navigator.clipboard.writeText(input.value).then(() => {
        const originalText = $t('copy');
        copyButtonText[elementId] = $t('copied');
        copyButtonText = copyButtonText; // trigger reactivity
        showToast($t('copiedToClipboard'));
        setTimeout(() => {
          copyButtonText[elementId] = originalText;
          copyButtonText = copyButtonText; // trigger reactivity
        }, 2000);
      }).catch(err => {
        console.error('Failed to copy text: ', err);
      });
    }
  }

  let toastMessage = '';
  function showToast(message) {
    toastMessage = message;
    setTimeout(() => {
      toastMessage = '';
    }, 3000);
  }
</script>

<div class="app-container">
  {#if toastMessage}
  <div class="toast" transition:fade={{ duration: 500 }}>
    {toastMessage}
  </div>
  {/if}

  <div class="title-bar" style="--wails-draggable:drag">
    <div class="title">{$t('title')}</div>
    <div class="window-controls">
      <button on:click={toggleLocale} class="lang-btn">{$locale === 'en' ? '中文' : 'EN'}</button>
      <button on:click={WindowMinimise}>−</button>
      <button on:click={WindowToggleMaximise}>□</button>
      <button on:click={Quit} class="close-btn">×</button>
    </div>
  </div>
  <main class="content-wrapper">
    <div class="container">


    <div class="info-grid">
      <div class="card">
        <p>{$t('configureStreaming')}</p>
        <ol class="steps">
            <li>
                <span>{$t('setServerUrl')}</span>
                <div class="input-group">
                    <input type="text" readonly value="rtmp://127.0.0.1:1935/live/" id="server-url">
                    <button on:click={() => copyToClipboard('server-url')}>{copyButtonText['server-url']}</button>
                </div>
            </li>
            <li>
                <span>{$t('setHtmlTitle')}</span>
                <div class="input-group">
                    <input type="text" placeholder={$t('enterNewTitle')} bind:value={htmlTitle}>
                    <button on:click={updateTitle}>{$t('update')}</button>
                </div>
            </li>
            <li>
                <span>{$t('useStreamKey')}</span>
                <div class="input-group">
                {#if streamKey === 'N/A'}
                    <input type="text" readonly value={$t('startToGetKey')} disabled>
                {:else}
                    <input type="text" readonly value={streamKey} id="stream-key">
                    <button on:click={() => copyToClipboard('stream-key')}>{copyButtonText['stream-key']}</button>
                {/if}
                </div>
            </li>
        </ol>
      </div>

    <div class="status-grid">
      {#each Object.entries(services) as [key, service]}
        <div class="service-card">
          <div class="service-info">
            <span class="status-light" class:running={service.status === 'Running'} class:stopped={service.status === 'Stopped'}></span>
            <div class="service-name">{$t(`${key}`)}</div>
          </div>
          <button on:click={key === 'backend' ? toggleBackend : toggleFRPC} class:running={service.status === 'Running'}>
            {service.status === 'Running' ? $t('stop') : $t('start')}
          </button>
        </div>
      {/each}
    </div>


      <div class="card logs-container">
        <h2 on:click={() => logsVisible = !logsVisible}>
          {$t('logs')} {logsVisible ? '▼' : '▶'}
        </h2>
        {#if logsVisible}
          <textarea readonly bind:this={logTextarea}>{logs}</textarea>
        {/if}
      </div>
      </div>
    </div>
  </main>
</div>

<style>
  :root {
    --bg-color: #f5f5f7;
    --card-color: #ffffff;
    --text-color: #1d1d1f;
    --text-alt-color: #6e6e73;
    --primary-color: #007aff;
    --green-color: #34c759;
    --red-color: #ff3b30;
    --border-color: #d2d2d7;
    --font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
  }

  .toast {
    position: fixed;
    top: 20px;
    left: 50%;
    transform: translateX(-50%);
    background-color: var(--primary-color);
    color: white;
    padding: 10px 20px;
    border-radius: 5px;
    z-index: 1000;
  }

  .app-container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background-color: var(--bg-color);
    color: var(--text-color);
    font-family: var(--font-family);
  }

  .content-wrapper {
    flex-grow: 1;
    overflow-y: auto;
    padding: 2em;
  }

  .title-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    height: 40px;
    background-color: #f5f5f7; /* Match the overall background */
    color: var(--text-color);
    flex-shrink: 0; /* Prevent title bar from shrinking */
  }

  .title {
    padding-left: 1em;
    font-weight: bold;
  }

  .window-controls {
    display: flex;
    height: 100%;
  }

  .window-controls button {
    width: 45px;
    height: 100%;
    border: none;
    background-color: transparent;
    color: var(--text-color);
    font-size: 1.2em;
    cursor: pointer;
    transition: background-color 0.2s;
  }

  .window-controls .lang-btn {
    font-size: 1em; /* Adjust font size for language button */
  }

  .window-controls button:hover {
    background-color: var(--border-color);
  }

  .window-controls .close-btn:hover {
    background-color: var(--red-color);
  }

  /* Custom Scrollbar */
  .content-wrapper::-webkit-scrollbar {
    width: 8px;
  }

  .content-wrapper::-webkit-scrollbar-track {
    background: var(--bg-color);
  }

  .content-wrapper::-webkit-scrollbar-thumb {
    background: var(--border-color);
    border-radius: 4px;
  }

  .content-wrapper::-webkit-scrollbar-thumb:hover {
    background: var(--primary-color);
  }

  .container {
    max-width: 800px;
    margin: 0 auto;
  }

  header {
    text-align: center;
    margin-bottom: 1.5em;
  }

  h1 {
    font-size: 2.5em;
    color: var(--text-alt-color);
    margin-bottom: 0.2em;
  }

  h2 {
      margin-top: 0;
  }

  header p {
      font-size: 1.1em;
      color: var(--text-alt-color);
  }

  .card {
      background-color: var(--card-color);
      border: 1px solid var(--border-color);
      padding: 1.5em;
      border-radius: 8px;
  }

  .status-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 1.5em;
    margin-bottom: 0.8em;
  }
  
  .service-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1em 1.5em;
    background-color: var(--card-color);
    border-radius: 8px;
    border: 1px solid var(--border-color);
  }

  .service-info {
    display: flex;
    align-items: center;
    gap: 0.7em;
  }

  .service-name {
      font-weight: bold;
      font-size: 1.1em;
      color: var(--text-alt-color);
  }

  .status-light {
    width: 14px;
    height: 14px;
    border-radius: 50%;
    display: inline-block;
    transition: background-color 0.3s ease;
  }

  .running .status-light, .status-light.running {
    background-color: var(--green-color);
    box-shadow: 0 0 8px var(--green-color);
  }

  .stopped .status-light, .status-light.stopped {
    background-color: var(--red-color);
  }

  .service-card button {
    padding: 0.5em 1.2em;
    font-size: 0.9em;
    font-weight: bold;
    cursor: pointer;
    border-radius: 5px;
    border: none;
    color: #fff;
    background-color: var(--primary-color);
    transition: background-color 0.3s ease;
    min-width: 70px;
    text-align: center;
  }

  .service-card button.running {
      background-color: var(--red-color);
  }
  
  .service-card button:hover {
      transform: translateY(-2px);
  }

  .info-grid {
      display: grid;
      grid-template-columns: 1fr;
      gap: 1.5em;
      margin-bottom: 1em;
    }
  
    .steps {
    list-style: none;
    padding: 0;
    margin-top: 1.5em;
  }

  .steps li {
    margin-bottom: 1.5em;
  }

  .steps li span {
    display: block;
    margin-bottom: 0.5em;
    color: var(--text-alt-color);
    font-weight: bold;
  }
  
  .input-group {
    display: flex;
  }

  .input-group input {
    flex-grow: 1;
    background-color: #f0f0f0; /* Slightly different shade for input background */
    border: 1px solid var(--border-color);
    color: var(--text-color);
    padding: 0.6em 0.8em;
    border-radius: 5px 0 0 5px;
    font-family: 'Fira Code', monospace;
    font-size: 0.95em;
    border-right: none;
  }
  
  .input-group input:focus {
      outline: none;
  }

  .input-group input:disabled {
    background-color: #e9e9e9;
    color: var(--text-alt-color);
  }

  .input-group button {
    background-color: var(--primary-color);
    color: white;
    border: none;
    padding: 0.6em 1.2em;
    border-radius: 0 5px 5px 0;
    cursor: pointer;
    font-weight: bold;
    transition: background-color 0.2s, color 0.2s;
    min-width: 80px;
    text-align: center;
  }

  .input-group button:hover {
    background-color: #0056b3; /* A slightly darker shade of primary for hover */
  }

  .logs-container h2 {
    cursor: pointer;
    user-select: none;
    margin-bottom: 0.5em;
    font-size: 1.2em;
  }

  textarea {
    width: 100%;
    height: 125px;
    font-family: 'Fira Code', monospace;
    font-size: 0.9em;
    white-space: pre-wrap;
    background-color: #ffffff;
    border: 1px solid var(--border-color);
    color: var(--text-color);
    border-radius: 5px;
    padding: 0.5em;
  }

  textarea::-webkit-scrollbar {
    width: 8px;
  }

  textarea::-webkit-scrollbar-track {
    background: var(--bg-color);
    border-radius: 4px;
  }

  textarea::-webkit-scrollbar-thumb {
    background: var(--border-color);
    border-radius: 4px;
  }

  textarea::-webkit-scrollbar-thumb:hover {
    background: var(--primary-color);
  }
</style>
