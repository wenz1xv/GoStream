import { writable, derived } from 'svelte/store';
import en from './en.json';
import zh from './zh.json';

const translations = { en, zh };

export const locale = writable('zh');

function translate(locale, key, vars) {
    let messages = translations[locale];
    if (!messages) {
        messages = translations['en'];
    }

    let text = messages[key] || key;
    
    if (text && typeof text === 'object' && vars.serviceName) {
        text = text[vars.serviceName.toLowerCase()] || text['default'] || key;
    }

    if (vars && Object.keys(vars).length > 0) {
        Object.keys(vars).forEach(k => {
            const regex = new RegExp(`{${k}}`, 'g');
            text = text.replace(regex, vars[k]);
        });
    }

    return text;
}

export const t = derived(locale, ($locale) => (key, vars = {}) => translate($locale, key, vars));