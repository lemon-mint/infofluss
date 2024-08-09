<script lang="ts">
  import DOMPurify from "dompurify";
  import { parse } from "marked";
  import { onMount } from "svelte";

  // Constants
  const TYPING_SPEED = 70;
  const DELETION_SPEED = 40;
  const PAUSE_BEFORE_DELETE = 1000;
  const PAUSE_BEFORE_TYPE = 400;
  const INITIAL_DELAY = 1500;
  const SUBMIT_COOLDOWN = 1000;

  // Types
  enum MessageType {
    Unknown = 0,
    Heartbeat = 1,
    Error = 2,
    QueryPlan = 3,
    SearchDone = 4,
    GenerateStream = 5,
    GenerateStreamDone = 6,
    CrawlDone = 7,
    SetSource = 8,
    Disconnect = 9,
  }

  interface QueryPlan {
    language: string;
    search_queries: SearchQuery[];
    instruction: string;
  }

  interface SearchQuery {
    query: string;
    description: string;
  }

  interface Message {
    type: MessageType;
    query_plan?: QueryPlan;
    success?: boolean;
    index?: number;
    text?: string;
    error?: string;
    url?: string;

    source?: Record<string, string>;
  }

  enum SearchState {
    InProgress,
    Done,
    Error,
  }

  // State
  let placeholder = "Ask a question";
  let query = "";
  let inputDisabled = false;
  let lastSubmit = 0;
  let searchResultsReady = false;
  let queryPlan: QueryPlan | null = null;
  let searchState: Array<SearchState> = [];
  let crawled: Array<string> = [];
  let source: Record<string, string> = {};
  let result_rendered = "";
  let result = "";
  let showSearchProcess = true;
  let isFirstToken = true;

  // Placeholder text handling
  const placeholderTexts = [
    "Ask anything!",
    "Let me know a good pasta restaurant near Seolleung Station!",
    "Holy Roman Empire misnomer",
    "NATS vs Kafka performance",
    "Best SvelteKit tutorials",
    "What is the airspeed velocity of an unladen swallow?",
    "How to make kimchi jjigae",
    "Quantum computing explained",
    "Top 10 travel destinations in Southeast Asia",
    "Best practices for writing clean code",
    "Explain the theory of relativity",
    "How to train a machine learning model",
    "History of the internet",
    "Benefits of meditation",
    "Latest advancements in artificial intelligence",
    "Best noise-canceling headphones",
    "How to bake sourdough bread",
    "Climate change solutions",
    "Learn a new language online",
    "Cryptocurrency explained",
    "DIY home improvement projects",
    "Best books of all time",
    "Mental health resources",
    "How to invest in stocks",
    "Sustainable living tips",
  ];

  let currentPlaceholderIndex = 0;
  let typingInterval: number;

  function shuffleArray(array: any[]) {
    for (let i = array.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [array[i], array[j]] = [array[j], array[i]];
    }
    return array;
  }

  shuffleArray(placeholderTexts);

  function typeText(text: string, index: number = 0) {
    if (index <= text.length) {
      placeholder = text.slice(0, index);
      typingInterval = setTimeout(
        () => typeText(text, index + 1),
        TYPING_SPEED
      );
    } else {
      setTimeout(deleteText, PAUSE_BEFORE_DELETE);
    }
  }

  function deleteText() {
    if (placeholder.length > 0) {
      placeholder = placeholder.slice(0, -1);
      typingInterval = setTimeout(deleteText, DELETION_SPEED);
    } else {
      currentPlaceholderIndex =
        (currentPlaceholderIndex + 1) % placeholderTexts.length;
      setTimeout(
        () => typeText(placeholderTexts[currentPlaceholderIndex]),
        PAUSE_BEFORE_TYPE
      );
    }
  }

  // Navigation
  function goToIndex() {
    window.location.href = "/";
  }

  // Search functionality
  async function search() {
    const response = await fetch("/api/v1/internal/search", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ query: query }),
    });

    const session_info = await response.json();
    const session_id = session_info.id;

    await readStream(session_id);
  }

  function readStream(session_id: string): Promise<void> {
    return new Promise((resolve, reject) => {
      const stream = new EventSource("/api/v1/internal/stream/" + session_id);
      stream.onerror = (error) => {
        console.log("Error: " + error);
        reject(error);
      };

      stream.onmessage = async (event) => {
        try {
          const data = JSON.parse(event.data) as Message;
          console.log(data);
          handleStreamMessage(data, stream, resolve, reject);
        } catch (e) {
          console.log(e);
          reject(e);
        }
      };
    });
  }

  function handleStreamMessage(
    data: Message,
    stream: EventSource,
    resolve: () => void,
    reject: (reason?: any) => void
  ) {
    switch (data.type) {
      case MessageType.QueryPlan:
        handleQueryPlan(data);
        break;
      case MessageType.SearchDone:
        handleSearchDone(data);
        break;
      case MessageType.GenerateStreamDone:
        console.log("GenerateStreamDone");
        break;
      case MessageType.CrawlDone:
        handleCrawlDone(data);
        break;
      case MessageType.GenerateStream:
        handleGenerateStream(data);
        break;
      case MessageType.SetSource:
        handleSetSource(data);
        break;
      case MessageType.Disconnect:
        stream.close();
        resolve();
        break;
      case MessageType.Error:
        handleError(data, reject, stream);
        break;
    }
  }

  function handleQueryPlan(data: Message) {
    queryPlan = data.query_plan!;
    searchState = Array(queryPlan.search_queries.length).fill(
      SearchState.InProgress
    );
    searchResultsReady = true;
    console.log(queryPlan);
  }

  function handleSearchDone(data: Message) {
    searchState[data.index ? data.index : 0] = data.success
      ? SearchState.Done
      : SearchState.Error;
  }

  function handleCrawlDone(data: Message) {
    console.log("Crawled: " + data.url);
    crawled = [...crawled, data.url!];
  }

  async function handleGenerateStream(data: Message) {
    if (isFirstToken) {
      isFirstToken = false;
      showSearchProcess = false;
    }

    if (data.text) {
      result += data.text;
      const parsed = await parse(result);
      result_rendered = DOMPurify.sanitize(parsed);
    }
    console.log(result);
  }

  function handleSetSource(data: Message) {
    console.log("Set source: " + data.source);
    source = data.source!;
  }

  function handleError(
    data: Message,
    reject: (reason?: any) => void,
    stream: EventSource
  ) {
    console.log(data.error);
    reject(data.error);
    stream.close();
  }

  function onFormSubmit(event: Event) {
    event.preventDefault();
    if (Date.now() - lastSubmit < SUBMIT_COOLDOWN || query === "") {
      return;
    }
    lastSubmit = Date.now();
    inputDisabled = true;

    resetSearchState();

    search().finally(() => {
      inputDisabled = false;
    });
  }

  function resetSearchState() {
    searchResultsReady = false;
    queryPlan = null;
    searchState = [];
    crawled = [];
    result_rendered = "";
    result = "";
    showSearchProcess = true;
    isFirstToken = true;
    source = {};
  }

  function toggleSearchProcess() {
    showSearchProcess = !showSearchProcess;
  }

  onMount(() => {
    (document.querySelector(".search-input") as HTMLInputElement).focus();

    setTimeout(() => {
      typeText(placeholderTexts[currentPlaceholderIndex]);
    }, INITIAL_DELAY);

    document.addEventListener("keyup", (event) => {
      if (event.key === "/") {
        (document.querySelector(".search-input") as HTMLInputElement).focus();
      }
    });

    if (location.search) {
      const q = new URLSearchParams(location.search).get("q");
      if (q) {
        query = q;
        history.replaceState(null, "", "/");
        onFormSubmit({ preventDefault: () => {} } as Event);
      }
    }

    return () => {
      clearTimeout(typingInterval);
    };
  });
</script>

<main class="info-fluss">
  <section class="hero-section">
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <span class="logo" on:click={goToIndex}>InfoFluss</span>

    <p class="tagline">Experience the flow of knowledge.</p>
    <form class="search-form" on:submit={onFormSubmit}>
      <label for="searchInput" class="visually-hidden">Search Query</label>
      <input
        type="text"
        id="searchInput"
        class="search-input"
        {placeholder}
        bind:value={query}
        aria-label="Search Query"
        disabled={inputDisabled}
        autocomplete="off"
      />
    </form>

    <div class="search-results">
      {#if searchResultsReady && queryPlan}
        <div class="card query-plan">
          <div class="card-header">
            <h2>Query Plan</h2>
            <button on:click={toggleSearchProcess} class="toggle-button">
              {showSearchProcess ? "Hide" : "Show"}
            </button>
          </div>
          {#if showSearchProcess}
            {#if queryPlan.search_queries.length !== 0}
              {#each queryPlan.search_queries as item, index}
                <div class="query-plan-item">
                  <span>
                    {searchState[index] === SearchState.InProgress
                      ? "🔍 Searching: "
                      : searchState[index] === SearchState.Done
                        ? "✅ Searched: "
                        : "😭 Search Failed: "}
                    {item.query}
                  </span>
                </div>
              {/each}
            {/if}
          {/if}
        </div>
      {/if}

      {#if crawled.length > 0}
        <div class="card crawled-pages">
          <div class="card-header">
            <h2>Crawled Pages</h2>
            <button on:click={toggleSearchProcess} class="toggle-button">
              {showSearchProcess ? "Hide" : "Show"}
            </button>
          </div>
          {#if showSearchProcess}
            {#if crawled.length !== 0 && !Object.entries(source).length}
              {#each crawled as item}
                <div class="crawled-item">
                  <a href={item}>🌐 {item}</a>
                </div>
              {/each}
            {/if}
            {#if !!Object.entries(source).length}
              {#each Object.keys(source) as index}
                <div class="crawled-item">
                  <a href={source[index]}>🌐 {index}. {source[index]}</a>
                </div>
              {/each}
            {/if}
          {/if}
        </div>
      {/if}

      {#if result_rendered}
        <div class="card search-results-container">
          {@html result_rendered}
        </div>
      {/if}
    </div>
  </section>
</main>

<style>
  .info-fluss {
    background-color: #f0e8ff;
    font-family: "IBM Plex Sans KR", sans-serif;
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .hero-section {
    width: 80vw;
    padding: 2em;
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .logo {
    font-size: 2em;
    font-weight: bold;
    color: #000;
    align-self: flex-start;
    cursor: pointer;
  }

  .tagline {
    font-size: 1.2em;
    color: #414141;
    margin: 2em 0;
  }

  .search-form {
    width: 100%;
    margin-bottom: 2em;
  }

  .search-input {
    width: 100%;
    padding: 0.5em;
    font-size: 1em;
    border: none;
    border-radius: 4px;
    background-color: rgba(16, 18, 53, 0.7);
    color: #fff;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  }

  .search-input:disabled {
    background-color: rgba(16, 18, 53, 0.3);
  }

  .search-results {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 1em;
  }

  .card {
    background-color: white;
    border-radius: 8px;
    padding: 1em;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
    width: 100%;
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5em;
  }

  .card h2 {
    font-size: 1.2em;
    color: #333;
    margin: 0;
  }

  .toggle-button {
    background-color: #6116c3;
    color: white;
    border: none;
    padding: 0.5em 1em;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.9em;
  }

  .query-plan-item,
  .crawled-item {
    margin-bottom: 0.5em;
    font-size: 0.9em;
    color: #555;
    line-break: anywhere;
  }

  .crawled-item a {
    color: #0066cc;
    text-decoration: none;
  }

  .crawled-item a:hover {
    text-decoration: underline;
  }

  .search-results-container {
    width: 100%;
    overflow-wrap: break-word;
  }

  .visually-hidden {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  @media (max-width: 600px) {
    .hero-section {
      padding: 1em;
    }

    .logo {
      font-size: 1.5em;
    }

    .tagline {
      font-size: 1em;
    }

    .card {
      min-width: 100%;
    }
  }
</style>
