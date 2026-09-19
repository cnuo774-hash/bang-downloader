import { createApp } from 'vue'
import { DynamicScroller, DynamicScrollerItem } from 'vue-virtual-scroller'
import 'vue-virtual-scroller/dist/vue-virtual-scroller.css'
import App from './App.vue'
import './style.css'

const app = createApp(App)
app.component('DynamicScroller', DynamicScroller)
app.component('DynamicScrollerItem', DynamicScrollerItem)
app.mount('#app')
