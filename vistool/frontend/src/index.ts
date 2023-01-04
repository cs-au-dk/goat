import cytoscape from "cytoscape";
// import coseBilkent from "cytoscape-cose-bilkent";
// import fcose from "cytoscape-fcose";
import CytoscapeDotLayout from "./CytoscapeDotLayout";

//cytoscape.use(coseBilkent);
//cytoscape.use(fcose);
cytoscape.use(CytoscapeDotLayout);

const url = "http://localhost:8080";

const BaseStyle: cytoscape.Stylesheet[] = [
  {
    selector: "node",
    style: {
      shape: "rectangle",
      "background-color": "#f0fff0",
      "text-wrap": "wrap",
      "border-color": "gray",
      "border-width": "1px",
    },
  },
  {
    selector: "node[str]",
    style: {
      label: "data(str)",
    },
  },
  {
    selector: "node > node",
    style: {
      height: "label",
      width: "label",
      padding: "10px",
      "text-valign": "center",
      //visibility: "visible",
    } as cytoscape.Css.Node,
  },
  {
    selector: "edge",
    style: {
      width: 3,
      color: "black",
      "target-arrow-color": "black",
      "target-arrow-shape": "triangle",
      "curve-style": "bezier",
      "control-point-step-size": 1,
    },
  },
];

const SGStyle: cytoscape.Stylesheet[] = BaseStyle.concat([
  {
    selector: ":parent",
    style: {
      "background-color": (ele) => (ele.data("blocks") ? "#CC0000" : "#FFD581"),
      //"background-opacity": 0.33,
    },
  },
]);

const IGStyle: cytoscape.Stylesheet[] = BaseStyle.concat([
  {
    selector: ":parent",
    style: {
      "background-color": "#e6ffff",
    },
  },
]);

addEventListener("DOMContentLoaded", async () => {
  document.getElementById("sbtn")!.addEventListener("click", async () => {
    const response = await fetch(url + "/shutdown");
    console.assert(response.ok);
  });

  const response = await fetch(url + "/graph");
  console.assert(response.ok);
  const SGData = await response.json();
  const container = document.getElementById("cy")!;

  const SGCy = cytoscape({
    container: container,
    style: SGStyle,
    wheelSensitivity: 0.2,
  });
  const eles = SGCy.add(SGData);
  //console.log(relativeConstraints);

  /*
  const layout = cy.layout({
    name: "cose-bilkent",
    nodeDimensionsIncludeLabels: true,
    animate: false,
    fit: true,
  } as any);
  */
  /*
  const relativeConstraints = eles
    .parents()
    .map((par) => {
      const ids = par
        .children()
        .map((child) => child.attr("id"))
        .sort();
      return ids.slice(1).map((c1, i) => ({ right: ids[i], left: c1, gap: 1 }));
    })
    .flat();
  const confConstraints = eles
    .parents()
    .map((par) => par.children().map((child) => child.id()));
  const layout = cy.layout({
    name: "fcose",
    nodeDimensionsIncludeLabels: true,
    animate: false,
    numIter: 10000,
    fit: true,
    sampleSize: 100,
    relativePlacementConstraint: relativeConstraints,
    alignmentConstraint: {
      horizontal: confConstraints,
    },
  } as any);
  */
  const layout = SGCy.layout({ name: "dot" });
  layout.run();
  setTimeout(() => SGCy.fit(SGCy.nodes()), 0);

  console.log(SGCy.edges().first());

  console.log("Loaded");

  const btn = document.getElementById("btn")!;
  btn.style.display = "none";

  SGCy.on("dbltap", "node", async (event) => {
    const ele = event.target;
    if (!ele.isChild()) {
      console.log("Only works for children");
    } else {
      const response = await fetch(url + `/intraproc?id=${ele.id()}`);
      console.assert(response.ok);
      const IGData = await response.json();

      SGCy.unmount();

      const IGCy = cytoscape({
        container: container,
        elements: IGData,
        style: IGStyle,
        wheelSensitivity: 0.2,
      });

      const layout = IGCy.layout({ name: "dot" });
      layout.run();
      setTimeout(() => IGCy.fit(IGCy.nodes()), 0);

      btn.style.display = "initial";
      btn.addEventListener(
        "click",
        () => {
          IGCy.unmount();
          IGCy.destroy();
          SGCy.mount(container);
          btn.style.display = "none";
        },
        { once: true }
      );
    }
  });
});
