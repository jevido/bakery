import { mount } from 'svelte'
import '@fontsource-variable/inter'
import './app.css'
import '@bakery/ui/theme'
import App from './App.svelte'

mount(App, { target: document.getElementById('app')! })
