use std::collections::HashMap;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum NodeType {
	Static,
	Dynamic,
	Splat,
}

#[derive(Clone, Debug)]
pub(crate) struct SegmentNode {
	pub(crate) pattern: String,
	pub(crate) node_type: NodeType,
	pub(crate) children: HashMap<String, SegmentNode>,
	pub(crate) dyn_children: Vec<SegmentNode>,
	pub(crate) param_name: String,
	pub(crate) final_score: i32,
}

impl Default for SegmentNode {
	fn default() -> Self {
		Self::new(NodeType::Static)
	}
}

impl SegmentNode {
	fn new(node_type: NodeType) -> Self {
		Self {
			pattern: String::new(),
			node_type,
			children: HashMap::new(),
			dyn_children: Vec::new(),
			param_name: String::new(),
			final_score: 0,
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
