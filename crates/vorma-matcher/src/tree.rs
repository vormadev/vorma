use std::sync::Arc;

use rustc_hash::FxHashMap;

use crate::pattern::Pattern;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum NodeType {
	Static,
	Dynamic,
	Splat,
}

#[derive(Debug)]
pub(crate) struct SegmentNode {
	// The pattern registered exactly at this tree position, if any. The
	// walks read candidates straight off the node; no store lookup.
	pub(crate) registered: Option<Arc<Pattern>>,
	pub(crate) node_type: NodeType,
	pub(crate) children: FxHashMap<String, SegmentNode>,
	pub(crate) dyn_children: Vec<SegmentNode>,
	pub(crate) param_name: String,
}

// Route trees can be arbitrarily deep, so neither cloning nor dropping
// may recurse; both walk with explicit stacks.
impl Clone for SegmentNode {
	fn clone(&self) -> Self {
		struct Record<'a> {
			source: &'a SegmentNode,
			static_children: Vec<(&'a str, usize)>,
			dyn_children: Vec<usize>,
		}

		fn record(source: &SegmentNode) -> Record<'_> {
			Record {
				source,
				static_children: Vec::new(),
				dyn_children: Vec::new(),
			}
		}

		let mut records = vec![record(self)];
		let mut stack = vec![0usize];
		while let Some(index) = stack.pop() {
			let source = records[index].source;
			for (key, child) in &source.children {
				let child_index = records.len();
				records.push(record(child));
				records[index].static_children.push((key, child_index));
				stack.push(child_index);
			}
			for child in &source.dyn_children {
				let child_index = records.len();
				records.push(record(child));
				records[index].dyn_children.push(child_index);
				stack.push(child_index);
			}
		}

		// Children always carry larger record indexes than their parent,
		// so a descending pass builds every child before its parent.
		let mut clones: Vec<Option<SegmentNode>> = (0..records.len()).map(|_| None).collect();
		for index in (0..records.len()).rev() {
			let rec = &records[index];
			let mut node = SegmentNode {
				registered: rec.source.registered.clone(),
				node_type: rec.source.node_type,
				children: FxHashMap::with_capacity_and_hasher(
					rec.static_children.len(),
					Default::default(),
				),
				dyn_children: Vec::with_capacity(rec.dyn_children.len()),
				param_name: rec.source.param_name.clone(),
			};
			for (key, child_index) in &rec.static_children {
				let child = clones[*child_index]
					.take()
					.expect("child clones are built before their parent");
				node.children.insert((*key).to_owned(), child);
			}
			for child_index in &rec.dyn_children {
				let child = clones[*child_index]
					.take()
					.expect("child clones are built before their parent");
				node.dyn_children.push(child);
			}
			clones[index] = Some(node);
		}
		clones[0].take().expect("the root clone is built last")
	}
}

impl Default for SegmentNode {
	fn default() -> Self {
		Self::new(NodeType::Static)
	}
}

impl SegmentNode {
	fn new(node_type: NodeType) -> Self {
		Self {
			registered: None,
			node_type,
			children: FxHashMap::default(),
			dyn_children: Vec::new(),
			param_name: String::new(),
		}
	}

	pub(crate) fn find_or_create_child(&mut self, seg: &str) -> &mut SegmentNode {
		if seg.is_empty() {
			return self
				.children
				.entry(String::new())
				.or_insert_with(|| SegmentNode::new(NodeType::Static));
		}

		match seg.as_bytes()[0] {
			b':' => {
				let param_name = &seg[1..];
				if let Some(pos) = self.dyn_children.iter().position(|child| {
					child.node_type == NodeType::Dynamic && child.param_name == param_name
				}) {
					return &mut self.dyn_children[pos];
				}
				let mut child = SegmentNode::new(NodeType::Dynamic);
				child.param_name = param_name.to_string();
				self.dyn_children.push(child);
				let len = self.dyn_children.len();
				&mut self.dyn_children[len - 1]
			}
			b'*' => {
				if let Some(pos) = self
					.dyn_children
					.iter()
					.position(|child| child.node_type == NodeType::Splat)
				{
					return &mut self.dyn_children[pos];
				}
				self.dyn_children.push(SegmentNode::new(NodeType::Splat));
				let len = self.dyn_children.len();
				&mut self.dyn_children[len - 1]
			}
			_ => self
				.children
				.entry(seg.to_string())
				.or_insert_with(|| SegmentNode::new(NodeType::Static)),
		}
	}
}

impl Drop for SegmentNode {
	fn drop(&mut self) {
		let mut stack = Vec::new();
		stack.extend(self.children.drain().map(|(_, child)| child));
		stack.append(&mut self.dyn_children);

		while let Some(mut node) = stack.pop() {
			stack.extend(node.children.drain().map(|(_, child)| child));
			stack.append(&mut node.dyn_children);
		}
	}
}
