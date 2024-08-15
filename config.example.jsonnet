{
  search_engines: [
    'google',
    'gon',
    'bing',
    'naver',
  ],
  search_endpoints: [
    'http://search_server:8080/search',
  ],
  providers: [
    {
      name: 'vertexai',
      type: 'vertexai',
      project_id: std.extVar('ENV_PROJECT_ID'),
      location: std.extVar('ENV_LOCATION'),
    },
    {
      name: 'aistudio',
      type: 'aistudio',
      api_key: std.extVar('ENV_AISTUDIO_API_KEY'),
    },
    {
      name: 'openai',
      type: 'openai',
      api_key: std.extVar('ENV_OPENAI_API_KEY'),
    },
    {
      name: 'groq',
      type: 'openai',
      api_key: std.extVar('ENV_GROQ_API_KEY'),
      baseurl: 'https://api.groq.com/openai/v1',
    },
  ],
  model_configs: {
    chat: {
      provider: 'aistudio',
      model: 'gemini-1.5-flash-001',
      parameters: {
        temperature: 1.0,
      },
    },
    query_planner: $.model_configs.chat,
    search_reranker: $.model_configs.chat,
    response_generator: {
      provider: 'vertexai',
      model: 'gemini-experimental',
      parameters: {
        temperature: 0.85,
      },
    },
  },
  crawler_configs: {
    mode: 'cdp',
  },
}
