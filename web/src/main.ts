import { createApp } from "vue";
import App from "./App.vue";
import router from "./router";
import { installFetchGuard } from "./session";

import "./style.css";

installFetchGuard(router);
createApp(App).use(router).mount("#app");

