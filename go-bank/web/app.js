'use strict';
const $ = (selector) => document.querySelector(selector);
let dashboard;
let manifest = '';
const money = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' });
const dates = new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric' });

async function api(path, options) {
  const response = await fetch(path, options);
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || `Request failed (${response.status}).`);
  return data;
}
function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}
function openTab(id, focus = false) {
  document.querySelectorAll('[data-tab]').forEach((button) => {
    const selected = button.dataset.tab === id;
    button.classList.toggle('active', selected);
    button.setAttribute('aria-selected', String(selected));
    button.tabIndex = selected ? 0 : -1;
    document.getElementById(button.dataset.tab).hidden = !selected;
    if (selected && focus) button.focus();
  });
}
document.querySelectorAll('[data-tab]').forEach((button, index, tabs) => {
  button.addEventListener('click', () => openTab(button.dataset.tab));
  button.addEventListener('keydown', (event) => {
    let next;
    if (event.key === 'ArrowRight') next = (index + 1) % tabs.length;
    if (event.key === 'ArrowLeft') next = (index - 1 + tabs.length) % tabs.length;
    if (event.key === 'Home') next = 0;
    if (event.key === 'End') next = tabs.length - 1;
    if (next !== undefined) { event.preventDefault(); openTab(tabs[next].dataset.tab, true); }
  });
});
$('[data-open-libraries]').addEventListener('click', () => openTab('libraries', true));

async function loadDashboard() {
  dashboard = await api('/api/dashboard');
  $('#customer').textContent = dashboard.customer.split(' ')[0];
  $('#total-balance').textContent = money.format(dashboard.totalBalance);
  dashboard.accounts.forEach((account) => {
    document.getElementById(`${account.id}-balance`).textContent = money.format(account.balance);
  });
  $('#dashboard-error').hidden = true;
  $('#transfer-submit').disabled = false;
  renderTransactions();
}
function renderTransactions() {
  if (!dashboard) return;
  const query = $('#transaction-search').value.toLowerCase().trim();
  const account = $('#account-filter').value;
  const transactions = dashboard.transactions.filter((item) =>
    (account === 'all' || item.account === account || item.toAccount === account) &&
    `${item.description} ${item.category}`.toLowerCase().includes(query));
  const body = $('#transactions');
  body.replaceChildren();
  transactions.forEach((item) => {
    const row = element('tr');
    const description = element('td');
    const info = element('div', 'transaction-info');
    const kind = item.category === 'Income' ? 'income' : item.category === 'Transfer' ? 'transfer' : '';
    const icon = element('span', `transaction-icon ${kind}`, kind === 'income' ? '↓' : kind === 'transfer' ? '⇄' : '↗');
    icon.setAttribute('aria-hidden', 'true');
    const labels = element('div');
    labels.append(element('div', 'transaction-title', item.description));
    const names = { checking: 'Checking', savings: 'Savings' };
    const accountLabel = item.toAccount ? `${names[item.account]} → ${names[item.toAccount]}` : names[item.account];
    labels.append(element('div', 'transaction-category', `${item.category} · ${accountLabel}`));
    info.append(icon, labels);
    description.append(info);
    // Display incoming transfers as credits when viewing the destination account.
    const value = account === item.toAccount ? -Number(item.amount) : Number(item.amount);
    const amount = element('td', `amount ${value > 0 ? 'positive' : ''}`, `${value > 0 ? '+' : ''}${money.format(value)}`);
    row.append(description, element('td', 'date-cell', dates.format(new Date(item.date))), amount);
    body.append(row);
  });
  if (!transactions.length) {
    const row = element('tr');
    const cell = element('td', 'empty', 'No matching transactions.');
    cell.colSpan = 3;
    row.append(cell);
    body.append(row);
  }
  $('#transaction-summary').textContent = `${transactions.length} transaction${transactions.length === 1 ? '' : 's'} · Latest 100 retained · Transfers shown once`;
}
$('#transaction-search').addEventListener('input', renderTransactions);
$('#account-filter').addEventListener('change', renderTransactions);
$('#from-account').addEventListener('change', () => {
  $('#to-account').value = $('#from-account').value === 'checking' ? 'savings' : 'checking';
});
$('#to-account').addEventListener('change', () => {
  $('#from-account').value = $('#to-account').value === 'checking' ? 'savings' : 'checking';
});
$('#transfer-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  const button = $('#transfer-submit');
  const status = $('#transfer-status');
  const input = Object.fromEntries(new FormData(event.target));
  button.disabled = true;
  button.textContent = 'Transferring…';
  status.hidden = true;
  let completed = false;
  try {
    await api('/api/transfers', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
    completed = true;
    $('#transfer-amount').value = '';
    $('#transfer-memo').value = '';
    await loadDashboard();
    status.className = 'notice';
    status.textContent = `${money.format(input.amount)} transferred to ${input.to === 'savings' ? 'Goals Savings' : 'Everyday Checking'}.`;
  } catch (error) {
    status.className = 'notice error';
    status.textContent = completed ? 'Transfer completed, but balances could not refresh. Reload the page to see your updated balance.' : error.message;
  } finally {
    status.hidden = false;
    button.disabled = false;
    button.textContent = 'Transfer money ↗';
  }
});
async function loadDependencies() {
  const result = await api('/api/dependencies');
  $('#dependency-count').textContent = result.modules.length;
  $('#dependency-note').textContent = result.note;
  $('#runtime').textContent = `Runtime: ${result.goVersion} · ${result.modules.length} compiled modules`;
  const list = $('#dependency-list');
  list.replaceChildren();
  for (const module of result.modules) {
    const card = element('article', 'dependency-card');
    const parts = module.path.split('/');
    const name = /^v\d+$/.test(parts.at(-1)) ? parts.at(-2) : parts.at(-1);
    card.append(element('span', 'dependency-version', module.version), element('h3', '', name), element('div', 'dependency-path', module.path), element('p', '', module.purpose || 'Supporting application dependency.'));
    const button = element('button', 'text-button', 'View module details →');
    button.addEventListener('click', () => {
      $('#dependency-title').textContent = name;
      $('#dependency-details').textContent = JSON.stringify(module, null, 2);
      $('#dependency-dialog').showModal();
    });
    card.append(button);
    list.append(card);
  }
}
$('#close-dialog').addEventListener('click', () => $('#dependency-dialog').close());
$('#dependency-dialog').addEventListener('click', (event) => {
  if (event.target === event.currentTarget) {
    const bounds = event.currentTarget.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) event.currentTarget.close();
  }
});
$('#copy-module').addEventListener('click', async () => {
  try { await navigator.clipboard.writeText(manifest); $('#copy-module').textContent = 'Copied'; }
  catch { $('#copy-module').textContent = 'Select the manifest to copy'; }
});
loadDashboard().catch((error) => {
  $('#dashboard-error').hidden = false;
  $('#dashboard-error').textContent = `Could not load your accounts: ${error.message} Reload to try again.`;
  $('#transactions').replaceChildren();
});
loadDependencies().catch((error) => { $('#dependency-list').textContent = `Could not load modules: ${error.message}`; });
api('/api/go-mod').then((data) => { manifest = data.content; $('#module-content').textContent = manifest; })
  .catch((error) => { $('#module-content').textContent = error.message; });
