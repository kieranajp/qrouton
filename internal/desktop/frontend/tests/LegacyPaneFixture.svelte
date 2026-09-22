<script>
 import MarkdownPane from "../src/lib/panes/MarkdownPane.svelte";
 let doc=$state({text:"# Regular\n\nNormal document",format:"markdown",source:"ordinary.md",staleness:""});
</script>
<button onclick={()=>doc={...doc,text:"# Legacy\n\n[code][source]\n\n[source]: repo://github.com/team/repo/main.go?revision=unknown#L12",staleness:"unknown"}}>Show legacy</button>
<button onclick={()=>doc={...doc,text:"# Regular\n\nNormal document",staleness:""}}>Show regular</button>
<button onclick={()=>doc={...doc,text:"",staleness:""}}>Revoke</button>
<MarkdownPane {doc} id="legacy-fixture"/>
