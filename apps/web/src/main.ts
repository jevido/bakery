import { mount } from 'svelte'
import '@fontsource-variable/inter'
import './theme.css'
import './app.css'
import './lib/theme.svelte'
import App from './App.svelte'

mount(App, { target: document.getElementById('app')! })
