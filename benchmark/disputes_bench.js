import http from 'k6/http';
import { check } from 'k6';
import { Trend, Counter } from 'k6/metrics';

const ENV = (typeof __ENV !== 'undefined' && __ENV) ? __ENV : {};
const BASE_URL  = ENV.BASE_URL || 'http://localhost:3000';
const LIMIT     = Number(ENV.LIMIT || 1000);
const VUS       = Number(ENV.VUS || 4);
const DURATION  = ENV.DURATION || '20s';
const MAX_PAGES = Number(ENV.MAX_PAGES || 1000);

const flow1d  = new Trend('flow_last_1d_total_ms');
const flow30d = new Trend('flow_last_30d_total_ms');
const page1d  = new Trend('page_last_1d_ms');
const page30d = new Trend('page_last_30d_ms');
const pages1d = new Counter('pages_last_1d');
const pages30d= new Counter('pages_last_30d');

export const options = {
    scenarios: {
        last_1d:  { executor:'constant-vus', vus: VUS, duration: DURATION, exec:'last_1d'  },
        last_30d: { executor:'constant-vus', vus: VUS, duration: DURATION, exec:'last_30d' },
    },
    summaryTrendStats: ['avg','med','p(95)','p(99)','count'],
};

function iso(d){ return new Date(d).toISOString(); }
function lastNDays(n){ const to=new Date(); const from=new Date(to.getTime()-n*24*3600*1000); return {from:iso(from), to:iso(to)}; }
function qs(o){ const p=[]; for (const k in o) if (o[k]!=null) p.push(encodeURIComponent(k)+'='+encodeURIComponent(String(o[k]))); return p.join('&'); }

function paginate(range){
    let cursor=null, pages=0, total=0;
    while (pages < MAX_PAGES) {
        const params = { limit: LIMIT, time_from: range.from, time_to: range.to };
        if (cursor) params.cursor = cursor;
        const res = http.get(`${BASE_URL}/disputes/events?${qs(params)}`);
        total += res.timings.duration; pages++;
        check(res, { '200': r => r.status === 200 });
        const b = res.json();
        if (!b || !b.has_more || !b.next_cursor) break;
        cursor = b.next_cursor;
    }
    return { total, pages };
}

export function last_1d (){ const r=lastNDays(1);  const {total,pages}=paginate(r); page1d.add(total/pages);  flow1d.add(total);  pages1d.add(pages); }
export function last_30d(){ const r=lastNDays(30); const {total,pages}=paginate(r); page30d.add(total/pages); flow30d.add(total); pages30d.add(pages); }
