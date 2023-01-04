// Source: https://gist.github.com/maccesch/791843a0c6c125f6b04b16106a29e4a0

import cytoscape from "cytoscape";
import Viz from "viz.js";
import { Module, render } from "viz.js/full.render.js";

function CytoscapeDotLayout(this: any, options: any) {
  this.options = options;
}

CytoscapeDotLayout.prototype.run = function () {
  let dotStr = "digraph G {\n\t nodesep=\"0.35\";\n";

  const addNode = (node: cytoscape.NodeSingular) => {
    const { w, h } = node.layoutDimensions(this.options);
    dotStr += `  "${node.id()}"[label="${node.id()}",fixedsize=true,width=${w / 100},height=${h / 100}];\n`;
  };

  const eles = this.options.eles;
  const nodes = eles.nodes();

  if (eles.parents().empty()) {
    nodes.forEach(addNode);
  } else {
    eles.parents().forEach((par: cytoscape.NodeSingular) => {
      dotStr += `\tsubgraph "cluster_${par.id()}" {\n`;
      par.children().forEach(addNode);
      dotStr += "\t}\n";
    });
  }

  const edges = this.options.eles.edges();
  for (let i = 0; i < edges.length; ++i) {
    const edge = edges[i];
    dotStr += `  "${edge.source().id()}" -> "${edge.target().id()}";\n`;
  }

  dotStr += "}";

  const viz = new Viz({ Module, render });

  //viz.renderJSONObject(dotStr).then(console.log);

  viz.renderSVGElement(dotStr).then((svg) => {
    const svgNodes = svg.getElementsByClassName("node");

    const idToPositions: any = {};

    let minY = Number.POSITIVE_INFINITY;

    for (let i = 0; i < svgNodes.length; ++i) {
      const node = svgNodes[i];

      const id = node.getElementsByTagName("title")[0].innerHTML.trim();

      const ellipse = node.getElementsByTagName("ellipse")[0];
      const y = ellipse.cy.baseVal.value * 2;

      idToPositions[id] = {
        x: ellipse.cx.baseVal.value * 2,
        y,
      };

      minY = Math.min(minY, y);
    }

    nodes.layoutPositions(this, this.options, (ele: any) => {
      let { x, y } = idToPositions[ele.id()];
      y -= minY - 30;
      return { x, y };
    });
  });

  return this;
};

export default function (cytoscape: any) {
  cytoscape("layout", "dot", CytoscapeDotLayout);
}
