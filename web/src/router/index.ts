import { createRouter, createWebHistory } from "vue-router";

const router = createRouter({
  history: createWebHistory("/"),
  routes: [
    { path: "/", name: "Home", component: () => import("../views/Home.vue") },
    {
      path: "/terminal",
      name: "Terminal",
      component: () => import("../views/Terminal.vue"),
    },
    {
      path: "/files",
      name: "Files",
      component: () => import("../views/Files.vue"),
    },
    {
      path: "/files/editor",
      name: "FileEditor",
      component: () => import("../views/FileEditor.vue"),
    },
    {
      path: "/database",
      name: "Database",
      component: () => import("../views/Database.vue"),
    },
    {
      path: "/database-mongo",
      name: "DatabaseMongo",
      component: () => import("../views/DatabaseMongo.vue"),
    },
    {
      path: "/redis",
      name: "Redis",
      component: () => import("../views/Redis.vue"),
    },
    {
      path: "/vnc",
      name: "Vnc",
      component: () => import("../views/Vnc.vue"),
    },
  ],
});

export default router;
